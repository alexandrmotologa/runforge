package durable

import (
	"time"
)

// RunStatus represents the lifecycle state of a run.
type RunStatus string

const (
	StatusPending         RunStatus = "pending"
	StatusRunning         RunStatus = "running"
	StatusPaused          RunStatus = "paused"
	StatusWaitingApproval RunStatus = "waiting_approval"
	StatusCompleted       RunStatus = "completed"
	StatusFailed          RunStatus = "failed"
)

// RunMode represents the execution strategy for an agent or workflow.
type RunMode string

const (
	ModeExploratory RunMode = "exploratory"
	ModeCompiled    RunMode = "compiled"
	ModeHybrid      RunMode = "hybrid"
)

// StepStatus represents execution state of an individual step.
type StepStatus string

const (
	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	StepSuccess StepStatus = "success"
	StepFailed  StepStatus = "failed"
	StepSkipped StepStatus = "skipped"
)

// StepType categorizes the action performed in a step.
type StepType string

const (
	StepTypeTool       StepType = "tool"
	StepTypeCommand    StepType = "command"
	StepTypeThought    StepType = "thought"
	StepTypeLLM        StepType = "llm"
	StepTypeCheckpoint StepType = "checkpoint"
)

// ApprovalStatus represents human-in-the-loop review state.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

// Run represents a complete agent execution session or compiled workflow invocation.
type Run struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	Name        string                 `json:"name"`
	Status      RunStatus              `json:"status"`
	Mode        RunMode                `json:"mode"`
	Model       string                 `json:"model"`
	Prompt      string                 `json:"prompt"`
	InputVars   map[string]interface{} `json:"input_vars"`
	TokensIn    int64                  `json:"tokens_in"`
	TokensOut   int64                  `json:"tokens_out"`
	TokensSaved int64                  `json:"tokens_saved"`
	CostUSD     float64                `json:"cost_usd"`
	CostSaved   float64                `json:"cost_saved"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Step records an atomic action, tool call, command, or observation in SQLite WAL.
type Step struct {
	ID            string     `json:"id"`
	RunID         string     `json:"run_id"`
	StepIndex     int        `json:"step_index"`
	Name          string     `json:"name"`
	Type          StepType   `json:"type"`
	ToolName      string     `json:"tool_name"`
	InputPayload  string     `json:"input_payload"`
	OutputPayload string     `json:"output_payload"`
	ExitCode      int        `json:"exit_code"`
	DurationMs    int64      `json:"duration_ms"`
	Status        StepStatus `json:"status"`
	IsPruned      bool       `json:"is_pruned"`
	CausalDeps    []string   `json:"causal_deps"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Checkpoint captures durable state enabling instant execution resumption.
type Checkpoint struct {
	ID         string                 `json:"id"`
	RunID      string                 `json:"run_id"`
	StepIndex  int                    `json:"step_index"`
	StateData  string                 `json:"state_data"`
	Variables  map[string]interface{} `json:"variables"`
	CreatedAt  time.Time              `json:"created_at"`
}

// Approval represents a guarded action requiring human confirmation.
type Approval struct {
	ID         string         `json:"id"`
	RunID      string         `json:"run_id"`
	StepIndex  int            `json:"step_index"`
	Command    string         `json:"command"`
	Reason     string         `json:"reason"`
	Status     ApprovalStatus `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	ResolvedAt *time.Time     `json:"resolved_at,omitempty"`
}

// AggregateStats summarizes token efficiency and execution totals.
type AggregateStats struct {
	TotalRuns        int64   `json:"total_runs"`
	ActiveRuns       int64   `json:"active_runs"`
	CompletedRuns    int64   `json:"completed_runs"`
	FailedRuns       int64   `json:"failed_runs"`
	TotalTokensIn    int64   `json:"total_tokens_in"`
	TotalTokensOut   int64   `json:"total_tokens_out"`
	TotalTokensSaved int64   `json:"total_tokens_saved"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	TotalCostSaved   float64 `json:"total_cost_saved"`
	AverageSavingsPct float64 `json:"average_savings_pct"`
}
