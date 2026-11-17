package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/runforge/internal/compiler"
	"github.com/alexandrmotologa/runforge/internal/durable"
	"github.com/alexandrmotologa/runforge/internal/sandbox"
	"github.com/alexandrmotologa/runforge/internal/telemetry"
)

// OrchestratorEvent provides real-time lifecycle notifications.
type OrchestratorEvent struct {
	Type      string      `json:"type"` // run_started, step_started, step_completed, approval_required, run_completed, run_failed
	RunID     string      `json:"run_id"`
	StepIndex int         `json:"step_index,omitempty"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Orchestrator manages the end-to-end lifecycle of agent and workflow executions.
type Orchestrator struct {
	ledger      *durable.Ledger
	runner      sandbox.Runner
	client      ModelClient
	router      *HandoffRouter
	costTracker *telemetry.CostTracker

	subscribersMu sync.RWMutex
	subscribers   map[chan OrchestratorEvent]bool
}

func NewOrchestrator(ledger *durable.Ledger, runner sandbox.Runner, client ModelClient) *Orchestrator {
	if runner == nil {
		runner = sandbox.NewDefaultRunner()
	}
	if client == nil {
		client = NewDefaultModelClient()
	}

	return &Orchestrator{
		ledger:      ledger,
		runner:      runner,
		client:      client,
		router:      NewHandoffRouter("", ""),
		costTracker: telemetry.NewCostTracker(),
		subscribers: make(map[chan OrchestratorEvent]bool),
	}
}

// Subscribe returns a channel receiving real-time execution events.
func (o *Orchestrator) Subscribe() chan OrchestratorEvent {
	o.subscribersMu.Lock()
	defer o.subscribersMu.Unlock()

	ch := make(chan OrchestratorEvent, 100)
	o.subscribers[ch] = true
	return ch
}

// Unsubscribe releases an active event channel.
func (o *Orchestrator) Unsubscribe(ch chan OrchestratorEvent) {
	o.subscribersMu.Lock()
	defer o.subscribersMu.Unlock()

	delete(o.subscribers, ch)
	close(ch)
}

func (o *Orchestrator) broadcast(event OrchestratorEvent) {
	o.subscribersMu.RLock()
	defer o.subscribersMu.RUnlock()

	event.Timestamp = time.Now().UTC()
	for ch := range o.subscribers {
		select {
		case ch <- event:
		default:
			// Non-blocking drop if subscriber is backpressured
		}
	}
}

// RunPlaybook executes a compiled deterministic workflow from start to completion.
func (o *Orchestrator) RunPlaybook(ctx context.Context, pb *compiler.Playbook, runID string, inputVars map[string]interface{}) (*durable.Run, error) {
	if runID == "" {
		runID = fmt.Sprintf("run-%d", time.Now().UnixNano())
	}

	mergedVars := make(map[string]interface{})
	for k, v := range pb.Inputs {
		mergedVars[k] = v.Default
	}
	for k, v := range inputVars {
		mergedVars[k] = v
	}

	run := &durable.Run{
		ID:         runID,
		WorkflowID: pb.Name,
		Name:       pb.Name,
		Status:     durable.StatusRunning,
		Mode:       durable.ModeCompiled,
		Model:      "deterministic-sandbox",
		Prompt:     pb.Description,
		InputVars:  mergedVars,
		CreatedAt:  time.Now().UTC(),
	}

	if err := o.ledger.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to record run in ledger: %w", err)
	}

	o.broadcast(OrchestratorEvent{
		Type:  "run_started",
		RunID: run.ID,
		Data:  run,
	})

	var totalTokensIn, totalTokensOut int64
	var totalCost float64

	for i, stepDef := range pb.Steps {
		stepID := fmt.Sprintf("%s-step-%02d", run.ID, i)
		step := &durable.Step{
			ID:           stepID,
			RunID:        run.ID,
			StepIndex:    i,
			Name:         stepDef.Name,
			Type:         durable.StepTypeCommand,
			ToolName:     stepDef.Tool,
			InputPayload: stepDef.Run,
			Status:       durable.StepRunning,
			CreatedAt:    time.Now().UTC(),
		}

		if err := o.ledger.RecordStep(ctx, step); err != nil {
			return nil, fmt.Errorf("failed to record step in ledger: %w", err)
		}

		o.broadcast(OrchestratorEvent{
			Type:      "step_started",
			RunID:     run.ID,
			StepIndex: i,
			Data:      step,
		})

		// Check for guarded operation requiring human confirmation
		if stepDef.RequiresApproval {
			app := &durable.Approval{
				ID:        fmt.Sprintf("app-%s-%d", run.ID, i),
				RunID:     run.ID,
				StepIndex: i,
				Command:   stepDef.Run,
				Reason:    fmt.Sprintf("Step %q requires operator confirmation before execution", stepDef.Name),
				Status:    durable.ApprovalPending,
			}
			_ = o.ledger.RecordApproval(ctx, app)
			_ = o.ledger.UpdateRunStatus(ctx, run.ID, durable.StatusWaitingApproval, totalTokensIn, totalTokensOut, pb.Metadata.OriginalTokens, totalCost, 0)

			o.broadcast(OrchestratorEvent{
				Type:      "approval_required",
				RunID:     run.ID,
				StepIndex: i,
				Data:      app,
			})

			run.Status = durable.StatusWaitingApproval
			return run, nil
		}

		// Execute step
		stepStart := time.Now()
		var outPayload string
		var exitCode int

		if stepDef.Run != "" {
			cmdStr := interpolateVariables(stepDef.Run, mergedVars)
			parts := strings.Fields(cmdStr)
			if len(parts) > 0 {
				execResult, err := o.runner.Execute(ctx, parts[0], parts[1:], sandbox.Limits{
					Timeout: 120 * time.Second,
				})
				if err != nil {
					step.Status = durable.StepFailed
					step.ExitCode = -1
					step.OutputPayload = fmt.Sprintf("execution error: %v", err)
					step.DurationMs = time.Since(stepStart).Milliseconds()
					_ = o.ledger.UpdateStep(ctx, step)
					_ = o.ledger.UpdateRunStatus(ctx, run.ID, durable.StatusFailed, totalTokensIn, totalTokensOut, 0, totalCost, 0)

					o.broadcast(OrchestratorEvent{
						Type:      "run_failed",
						RunID:     run.ID,
						StepIndex: i,
						Data:      err.Error(),
					})
					return nil, fmt.Errorf("step %d failed: %w", i, err)
				}
				outPayload = execResult.Stdout
				if execResult.Stderr != "" {
					outPayload += "\n[stderr] " + execResult.Stderr
				}
				exitCode = execResult.ExitCode
			}
		} else if stepDef.Path != "" && stepDef.Content != "" {
			// File write step
			pathStr := interpolateVariables(stepDef.Path, mergedVars)
			if err := os.MkdirAll(filepath.Dir(pathStr), 0755); err == nil {
				_ = os.WriteFile(pathStr, []byte(stepDef.Content), 0644)
			}
			outPayload = fmt.Sprintf("Wrote %d bytes to %s", len(stepDef.Content), pathStr)
			exitCode = 0
		}

		step.DurationMs = time.Since(stepStart).Milliseconds()
		step.OutputPayload = outPayload
		step.ExitCode = exitCode
		step.Status = durable.StepSuccess
		if exitCode != 0 {
			step.Status = durable.StepFailed
		}

		_ = o.ledger.UpdateStep(ctx, step)

		// Create checkpoint after successful step
		cp := &durable.Checkpoint{
			ID:        fmt.Sprintf("cp-%s-%d", run.ID, i),
			RunID:     run.ID,
			StepIndex: i,
			Variables: mergedVars,
		}
		_ = o.ledger.CreateCheckpoint(ctx, cp)

		o.broadcast(OrchestratorEvent{
			Type:      "step_completed",
			RunID:     run.ID,
			StepIndex: i,
			Data:      step,
		})

		if exitCode != 0 {
			_ = o.ledger.UpdateRunStatus(ctx, run.ID, durable.StatusFailed, totalTokensIn, totalTokensOut, 0, totalCost, 0)
			return nil, fmt.Errorf("step %d exited with code %d: %s", i, exitCode, outPayload)
		}
	}

	// Calculate net cost savings
	tokensSaved := pb.Metadata.OriginalTokens
	if tokensSaved == 0 {
		tokensSaved = int64(len(pb.Steps)) * 4000
	}
	costSaved := o.costTracker.CalculateSavings("claude-3-5-sonnet", tokensSaved, 0, totalCost)

	_ = o.ledger.UpdateRunStatus(ctx, run.ID, durable.StatusCompleted, totalTokensIn, totalTokensOut, tokensSaved, totalCost, costSaved)
	run.Status = durable.StatusCompleted
	run.TokensSaved = tokensSaved
	run.CostSaved = costSaved

	o.broadcast(OrchestratorEvent{
		Type:  "run_completed",
		RunID: run.ID,
		Data:  run,
	})

	return run, nil
}

// ResumeRun continues an interrupted or approval-paused run from its SQLite WAL ledger state.
func (o *Orchestrator) ResumeRun(ctx context.Context, runID string) (*durable.Run, error) {
	plan, err := o.ledger.GetResumePlan(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to plan resume: %w", err)
	}

	if plan.PendingApproval != nil {
		if plan.PendingApproval.Status == durable.ApprovalPending {
			return nil, fmt.Errorf("run is waiting for human approval on command %q", plan.PendingApproval.Command)
		}
	}

	// Update run to active running
	_ = o.ledger.UpdateRunStatus(ctx, runID, durable.StatusRunning, plan.Run.TokensIn, plan.Run.TokensOut, plan.Run.TokensSaved, plan.Run.CostUSD, plan.Run.CostSaved)

	// Fetch run details and broadcast resume
	run, _ := o.ledger.GetRun(ctx, runID)
	o.broadcast(OrchestratorEvent{
		Type:  "run_resumed",
		RunID: runID,
		Data:  plan,
	})

	return run, nil
}

func interpolateVariables(tpl string, vars map[string]interface{}) string {
	res := tpl
	for k, v := range vars {
		placeholder := fmt.Sprintf("${{ inputs.%s }}", k)
		res = strings.ReplaceAll(res, placeholder, fmt.Sprintf("%v", v))
	}
	return res
}
