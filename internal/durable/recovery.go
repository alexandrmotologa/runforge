package durable

import (
	"context"
	"fmt"
)

// ResumePlan describes how an interrupted run will be resumed without lost progress.
type ResumePlan struct {
	Run             *Run                   `json:"run"`
	LastStepIndex   int                    `json:"last_step_index"`
	NextStepIndex   int                    `json:"next_step_index"`
	CompletedSteps  int                    `json:"completed_steps"`
	TotalSteps      int                    `json:"total_steps"`
	LastCheckpoint  *Checkpoint            `json:"last_checkpoint,omitempty"`
	RestoredVars    map[string]interface{} `json:"restored_vars"`
	PendingApproval *Approval              `json:"pending_approval,omitempty"`
}

// FindInterruptedRuns identifies runs that were executing when an interruption occurred.
func (l *Ledger) FindInterruptedRuns(ctx context.Context) ([]*Run, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	query := `
	SELECT id, workflow_id, name, status, mode, model, prompt, input_vars, tokens_in, tokens_out, tokens_saved, cost_usd, cost_saved, created_at, updated_at
	FROM runs 
	WHERE status IN ('running', 'waiting_approval', 'paused')
	ORDER BY updated_at DESC;
	`

	rows, err := l.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query interrupted runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		var r Run
		var statusStr, modeStr, inputVarsStr string

		err := rows.Scan(
			&r.ID,
			&r.WorkflowID,
			&r.Name,
			&statusStr,
			&modeStr,
			&r.Model,
			&r.Prompt,
			&inputVarsStr,
			&r.TokensIn,
			&r.TokensOut,
			&r.TokensSaved,
			&r.CostUSD,
			&r.CostSaved,
			&r.CreatedAt,
			&r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		r.Status = RunStatus(statusStr)
		r.Mode = RunMode(modeStr)
		r.InputVars = make(map[string]interface{})
		runs = append(runs, &r)
	}

	return runs, rows.Err()
}

// GetResumePlan constructs state restoration parameters for an interrupted run.
func (l *Ledger) GetResumePlan(ctx context.Context, runID string) (*ResumePlan, error) {
	run, err := l.GetRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("run not found: %w", err)
	}

	steps, err := l.GetStepsByRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load steps for run %s: %w", runID, err)
	}

	checkpoint, _ := l.GetLatestCheckpoint(ctx, runID)

	pendingApprovals, err := l.GetPendingApprovals(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query approvals: %w", err)
	}

	var pending *Approval
	if len(pendingApprovals) > 0 {
		pending = pendingApprovals[0]
	}

	highestCompletedIndex := -1
	completedCount := 0

	for _, step := range steps {
		if step.Status == StepSuccess {
			completedCount++
			if step.StepIndex > highestCompletedIndex {
				highestCompletedIndex = step.StepIndex
			}
		}
	}

	restoredVars := make(map[string]interface{})
	if run.InputVars != nil {
		for k, v := range run.InputVars {
			restoredVars[k] = v
		}
	}
	if checkpoint != nil && checkpoint.Variables != nil {
		for k, v := range checkpoint.Variables {
			restoredVars[k] = v
		}
	}

	return &ResumePlan{
		Run:             run,
		LastStepIndex:   highestCompletedIndex,
		NextStepIndex:   highestCompletedIndex + 1,
		CompletedSteps:  completedCount,
		TotalSteps:      len(steps),
		LastCheckpoint:  checkpoint,
		RestoredVars:    restoredVars,
		PendingApproval: pending,
	}, nil
}
