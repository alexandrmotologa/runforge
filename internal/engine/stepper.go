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
)

// StepperStatus captures current debugging position and state variables.
type StepperStatus struct {
	RunID            string                 `json:"run_id"`
	CurrentStepIndex int                    `json:"current_step_index"`
	TotalSteps       int                    `json:"total_steps"`
	IsComplete       bool                   `json:"is_complete"`
	Variables        map[string]interface{} `json:"variables"`
	NextStep         *compiler.PlaybookStep `json:"next_step,omitempty"`
}

// Stepper provides step-by-step interactive execution and state rewind capabilities.
type Stepper struct {
	ledger    *durable.Ledger
	runner    sandbox.Runner
	run       *durable.Run
	playbook  *compiler.Playbook
	stepIdx   int
	variables map[string]interface{}
	mu        sync.Mutex
}

// NewStepper initializes an interactive step-by-step debugging session.
func NewStepper(ledger *durable.Ledger, runner sandbox.Runner, pb *compiler.Playbook, runID string, initialVars map[string]interface{}) (*Stepper, error) {
	if runner == nil {
		runner = sandbox.NewDefaultRunner()
	}

	mergedVars := make(map[string]interface{})
	for k, v := range pb.Inputs {
		mergedVars[k] = v.Default
	}
	for k, v := range initialVars {
		mergedVars[k] = v
	}

	run := &durable.Run{
		ID:         runID,
		WorkflowID: pb.Name,
		Name:       pb.Name,
		Status:     durable.StatusRunning,
		Mode:       durable.ModeCompiled,
		Model:      "interactive-stepper",
		Prompt:     pb.Description,
		InputVars:  mergedVars,
		CreatedAt:  time.Now().UTC(),
	}

	if err := ledger.CreateRun(context.Background(), run); err != nil {
		return nil, fmt.Errorf("failed to create stepper run: %w", err)
	}

	return &Stepper{
		ledger:    ledger,
		runner:    runner,
		run:       run,
		playbook:  pb,
		stepIdx:   0,
		variables: mergedVars,
	}, nil
}

// StepNext advances execution by exactly one deterministic step and checkpoints state.
func (s *Stepper) StepNext(ctx context.Context) (*durable.Step, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stepIdx >= len(s.playbook.Steps) {
		_ = s.ledger.UpdateRunStatus(ctx, s.run.ID, durable.StatusCompleted, 0, 0, s.playbook.Metadata.OriginalTokens, 0, 0)
		return nil, fmt.Errorf("playbook execution complete (no more steps)")
	}

	stepDef := s.playbook.Steps[s.stepIdx]
	stepID := fmt.Sprintf("%s-step-%02d", s.run.ID, s.stepIdx)

	step := &durable.Step{
		ID:           stepID,
		RunID:        s.run.ID,
		StepIndex:    s.stepIdx,
		Name:         stepDef.Name,
		Type:         durable.StepTypeCommand,
		ToolName:     stepDef.Tool,
		InputPayload: stepDef.Run,
		Status:       durable.StepRunning,
		CreatedAt:    time.Now().UTC(),
	}
	_ = s.ledger.RecordStep(ctx, step)

	stepStart := time.Now()
	var outPayload string
	var exitCode int

	if stepDef.Run != "" {
		cmdStr := interpolateVariables(stepDef.Run, s.variables)
		parts := strings.Fields(cmdStr)
		if len(parts) > 0 {
			res, err := s.runner.Execute(ctx, parts[0], parts[1:], sandbox.Limits{
				Timeout: 60 * time.Second,
			})
			if err != nil {
				step.Status = durable.StepFailed
				step.ExitCode = -1
				step.OutputPayload = fmt.Sprintf("execution error: %v", err)
				_ = s.ledger.UpdateStep(ctx, step)
				return step, err
			}
			outPayload = res.Stdout
			if res.Stderr != "" {
				outPayload += "\n[stderr] " + res.Stderr
			}
			exitCode = res.ExitCode
		}
	} else if stepDef.Path != "" && stepDef.Content != "" {
		pathStr := interpolateVariables(stepDef.Path, s.variables)
		_ = os.MkdirAll(filepath.Dir(pathStr), 0755)
		_ = os.WriteFile(pathStr, []byte(stepDef.Content), 0644)
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
	_ = s.ledger.UpdateStep(ctx, step)

	// Create durable checkpoint
	cp := &durable.Checkpoint{
		ID:        fmt.Sprintf("cp-%s-%d", s.run.ID, s.stepIdx),
		RunID:     s.run.ID,
		StepIndex: s.stepIdx,
		Variables: s.variables,
	}
	_ = s.ledger.CreateCheckpoint(ctx, cp)

	s.stepIdx++
	if s.stepIdx >= len(s.playbook.Steps) {
		_ = s.ledger.UpdateRunStatus(ctx, s.run.ID, durable.StatusCompleted, 0, 0, s.playbook.Metadata.OriginalTokens, 0, 0)
	}

	return step, nil
}

// Rewind rolls back the execution pointer and variables to a previous SQLite checkpoint.
func (s *Stepper) Rewind(ctx context.Context, targetStepIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if targetStepIndex < 0 || targetStepIndex >= len(s.playbook.Steps) {
		return fmt.Errorf("invalid rewind target step index: %d", targetStepIndex)
	}

	// Fetch checkpoint for the target step
	cp, err := s.ledger.GetLatestCheckpoint(ctx, s.run.ID)
	if err == nil && cp != nil && cp.Variables != nil {
		s.variables = cp.Variables
	}

	s.stepIdx = targetStepIndex
	return nil
}

// SetVariable overrides or injects a variable dynamically before executing subsequent steps.
func (s *Stepper) SetVariable(key string, val interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.variables[key] = val
}

// Status returns the current stepper state.
func (s *Stepper) Status() *StepperStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	isComplete := s.stepIdx >= len(s.playbook.Steps)
	var next *compiler.PlaybookStep
	if !isComplete {
		next = s.playbook.Steps[s.stepIdx]
	}

	varsCopy := make(map[string]interface{})
	for k, v := range s.variables {
		varsCopy[k] = v
	}

	return &StepperStatus{
		RunID:            s.run.ID,
		CurrentStepIndex: s.stepIdx,
		TotalSteps:       len(s.playbook.Steps),
		IsComplete:       isComplete,
		Variables:        varsCopy,
		NextStep:         next,
	}
}
