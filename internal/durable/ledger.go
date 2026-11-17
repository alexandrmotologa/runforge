package durable

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Ledger coordinates transactional persistence with pure Go SQLite WAL.
type Ledger struct {
	db   *sql.DB
	path string
	mu   sync.RWMutex
}

// NewLedger initializes an embedded SQLite database configured for high-concurrency WAL mode.
func NewLedger(dbPath string) (*Ledger, error) {
	if dbPath == "" {
		dbPath = "runforge.db"
	}

	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for database: %w", err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	// Set connection pool parameters for SQLite WAL mode
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ledger := &Ledger{
		db:   db,
		path: dbPath,
	}

	if err := ledger.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return ledger, nil
}

// migrate creates tables and indexes if they do not exist.
func (l *Ledger) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		workflow_id TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL,
		status TEXT NOT NULL,
		mode TEXT NOT NULL,
		model TEXT NOT NULL DEFAULT '',
		prompt TEXT NOT NULL DEFAULT '',
		input_vars TEXT NOT NULL DEFAULT '{}',
		tokens_in INTEGER NOT NULL DEFAULT 0,
		tokens_out INTEGER NOT NULL DEFAULT 0,
		tokens_saved INTEGER NOT NULL DEFAULT 0,
		cost_usd REAL NOT NULL DEFAULT 0.0,
		cost_saved REAL NOT NULL DEFAULT 0.0,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
	CREATE INDEX IF NOT EXISTS idx_runs_created_at ON runs(created_at DESC);

	CREATE TABLE IF NOT EXISTS steps (
		id TEXT PRIMARY KEY,
		run_id TEXT NOT NULL,
		step_index INTEGER NOT NULL,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		tool_name TEXT NOT NULL DEFAULT '',
		input_payload TEXT NOT NULL DEFAULT '',
		output_payload TEXT NOT NULL DEFAULT '',
		exit_code INTEGER NOT NULL DEFAULT 0,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL,
		is_pruned BOOLEAN NOT NULL DEFAULT 0,
		causal_deps TEXT NOT NULL DEFAULT '[]',
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_steps_run_id_index ON steps(run_id, step_index ASC);

	CREATE TABLE IF NOT EXISTS checkpoints (
		id TEXT PRIMARY KEY,
		run_id TEXT NOT NULL,
		step_index INTEGER NOT NULL,
		state_data TEXT NOT NULL DEFAULT '',
		variables TEXT NOT NULL DEFAULT '{}',
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_checkpoints_run_id ON checkpoints(run_id, step_index DESC);

	CREATE TABLE IF NOT EXISTS approvals (
		id TEXT PRIMARY KEY,
		run_id TEXT NOT NULL,
		step_index INTEGER NOT NULL,
		command TEXT NOT NULL,
		reason TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		resolved_at TIMESTAMP,
		FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_approvals_run_status ON approvals(run_id, status);
	`

	_, err := l.db.Exec(schema)
	return err
}

// CreateRun records a new execution run in the SQLite ledger.
func (l *Ledger) CreateRun(ctx context.Context, run *Run) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	run.UpdatedAt = now

	varsJSON, err := json.Marshal(run.InputVars)
	if err != nil {
		varsJSON = []byte("{}")
	}

	query := `
	INSERT INTO runs (id, workflow_id, name, status, mode, model, prompt, input_vars, tokens_in, tokens_out, tokens_saved, cost_usd, cost_saved, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`

	_, err = l.db.ExecContext(ctx, query,
		run.ID,
		run.WorkflowID,
		run.Name,
		string(run.Status),
		string(run.Mode),
		run.Model,
		run.Prompt,
		string(varsJSON),
		run.TokensIn,
		run.TokensOut,
		run.TokensSaved,
		run.CostUSD,
		run.CostSaved,
		run.CreatedAt,
		run.UpdatedAt,
	)
	return err
}

// GetRun retrieves a run by its unique identifier.
func (l *Ledger) GetRun(ctx context.Context, runID string) (*Run, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	query := `
	SELECT id, workflow_id, name, status, mode, model, prompt, input_vars, tokens_in, tokens_out, tokens_saved, cost_usd, cost_saved, created_at, updated_at
	FROM runs WHERE id = ?;
	`

	row := l.db.QueryRowContext(ctx, query, runID)
	var r Run
	var statusStr, modeStr, inputVarsStr string

	err := row.Scan(
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
	_ = json.Unmarshal([]byte(inputVarsStr), &r.InputVars)

	return &r, nil
}

// ListRuns returns runs ordered by creation time descending.
func (l *Ledger) ListRuns(ctx context.Context, limit, offset int) ([]*Run, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query := `
	SELECT id, workflow_id, name, status, mode, model, prompt, input_vars, tokens_in, tokens_out, tokens_saved, cost_usd, cost_saved, created_at, updated_at
	FROM runs ORDER BY created_at DESC LIMIT ? OFFSET ?;
	`

	rows, err := l.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
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
		_ = json.Unmarshal([]byte(inputVarsStr), &r.InputVars)

		runs = append(runs, &r)
	}

	return runs, rows.Err()
}

// UpdateRunStatus modifies the lifecycle status and metric totals of a run.
func (l *Ledger) UpdateRunStatus(ctx context.Context, runID string, status RunStatus, tokensIn, tokensOut, tokensSaved int64, costUSD, costSaved float64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()
	query := `
	UPDATE runs SET status = ?, tokens_in = ?, tokens_out = ?, tokens_saved = ?, cost_usd = ?, cost_saved = ?, updated_at = ?
	WHERE id = ?;
	`

	_, err := l.db.ExecContext(ctx, query, string(status), tokensIn, tokensOut, tokensSaved, costUSD, costSaved, now, runID)
	return err
}

// RecordStep appends an atomic step to the run's audit trail.
func (l *Ledger) RecordStep(ctx context.Context, step *Step) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if step.CreatedAt.IsZero() {
		step.CreatedAt = time.Now().UTC()
	}

	depsJSON, err := json.Marshal(step.CausalDeps)
	if err != nil {
		depsJSON = []byte("[]")
	}

	query := `
	INSERT INTO steps (id, run_id, step_index, name, type, tool_name, input_payload, output_payload, exit_code, duration_ms, status, is_pruned, causal_deps, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`

	_, err = l.db.ExecContext(ctx, query,
		step.ID,
		step.RunID,
		step.StepIndex,
		step.Name,
		string(step.Type),
		step.ToolName,
		step.InputPayload,
		step.OutputPayload,
		step.ExitCode,
		step.DurationMs,
		string(step.Status),
		step.IsPruned,
		string(depsJSON),
		step.CreatedAt,
	)
	return err
}

// UpdateStep updates the execution result, duration, status, and payload of an existing step.
func (l *Ledger) UpdateStep(ctx context.Context, step *Step) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	depsJSON, err := json.Marshal(step.CausalDeps)
	if err != nil {
		depsJSON = []byte("[]")
	}

	query := `
	UPDATE steps SET output_payload = ?, exit_code = ?, duration_ms = ?, status = ?, is_pruned = ?, causal_deps = ?
	WHERE id = ?;
	`

	_, err = l.db.ExecContext(ctx, query,
		step.OutputPayload,
		step.ExitCode,
		step.DurationMs,
		string(step.Status),
		step.IsPruned,
		string(depsJSON),
		step.ID,
	)
	return err
}

// GetStepsByRun returns all steps recorded for a run, ordered by step_index.
func (l *Ledger) GetStepsByRun(ctx context.Context, runID string) ([]*Step, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	query := `
	SELECT id, run_id, step_index, name, type, tool_name, input_payload, output_payload, exit_code, duration_ms, status, is_pruned, causal_deps, created_at
	FROM steps WHERE run_id = ? ORDER BY step_index ASC;
	`

	rows, err := l.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []*Step
	for rows.Next() {
		var s Step
		var typeStr, statusStr, depsStr string

		err := rows.Scan(
			&s.ID,
			&s.RunID,
			&s.StepIndex,
			&s.Name,
			&typeStr,
			&s.ToolName,
			&s.InputPayload,
			&s.OutputPayload,
			&s.ExitCode,
			&s.DurationMs,
			&statusStr,
			&s.IsPruned,
			&depsStr,
			&s.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		s.Type = StepType(typeStr)
		s.Status = StepStatus(statusStr)
		_ = json.Unmarshal([]byte(depsStr), &s.CausalDeps)

		steps = append(steps, &s)
	}

	return steps, rows.Err()
}

// CreateCheckpoint persists durable runtime state for instant recovery.
func (l *Ledger) CreateCheckpoint(ctx context.Context, cp *Checkpoint) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}

	varsJSON, err := json.Marshal(cp.Variables)
	if err != nil {
		varsJSON = []byte("{}")
	}

	query := `
	INSERT INTO checkpoints (id, run_id, step_index, state_data, variables, created_at)
	VALUES (?, ?, ?, ?, ?, ?);
	`

	_, err = l.db.ExecContext(ctx, query,
		cp.ID,
		cp.RunID,
		cp.StepIndex,
		cp.StateData,
		string(varsJSON),
		cp.CreatedAt,
	)
	return err
}

// GetLatestCheckpoint returns the most recent checkpoint for a given run.
func (l *Ledger) GetLatestCheckpoint(ctx context.Context, runID string) (*Checkpoint, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	query := `
	SELECT id, run_id, step_index, state_data, variables, created_at
	FROM checkpoints WHERE run_id = ? ORDER BY step_index DESC LIMIT 1;
	`

	row := l.db.QueryRowContext(ctx, query, runID)
	var cp Checkpoint
	var varsStr string

	err := row.Scan(
		&cp.ID,
		&cp.RunID,
		&cp.StepIndex,
		&cp.StateData,
		&varsStr,
		&cp.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	cp.Variables = make(map[string]interface{})
	_ = json.Unmarshal([]byte(varsStr), &cp.Variables)

	return &cp, nil
}

// RecordApproval registers a human confirmation gate.
func (l *Ledger) RecordApproval(ctx context.Context, app *Approval) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if app.CreatedAt.IsZero() {
		app.CreatedAt = time.Now().UTC()
	}

	query := `
	INSERT INTO approvals (id, run_id, step_index, command, reason, status, created_at, resolved_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`

	_, err := l.db.ExecContext(ctx, query,
		app.ID,
		app.RunID,
		app.StepIndex,
		app.Command,
		app.Reason,
		string(app.Status),
		app.CreatedAt,
		app.ResolvedAt,
	)
	return err
}

// UpdateApproval records the resolution of a human approval gate.
func (l *Ledger) UpdateApproval(ctx context.Context, approvalID string, status ApprovalStatus) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()
	query := `
	UPDATE approvals SET status = ?, resolved_at = ? WHERE id = ?;
	`

	_, err := l.db.ExecContext(ctx, query, string(status), now, approvalID)
	return err
}

// GetApproval retrieves an approval gate by ID.
func (l *Ledger) GetApproval(ctx context.Context, approvalID string) (*Approval, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	query := `
	SELECT id, run_id, step_index, command, reason, status, created_at, resolved_at
	FROM approvals WHERE id = ?;
	`

	row := l.db.QueryRowContext(ctx, query, approvalID)
	var app Approval
	var statusStr string

	err := row.Scan(
		&app.ID,
		&app.RunID,
		&app.StepIndex,
		&app.Command,
		&app.Reason,
		&statusStr,
		&app.CreatedAt,
		&app.ResolvedAt,
	)
	if err != nil {
		return nil, err
	}

	app.Status = ApprovalStatus(statusStr)
	return &app, nil
}

// GetPendingApprovals returns unapproved gates for a run.
func (l *Ledger) GetPendingApprovals(ctx context.Context, runID string) ([]*Approval, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	query := `
	SELECT id, run_id, step_index, command, reason, status, created_at, resolved_at
	FROM approvals WHERE run_id = ? AND status = ? ORDER BY step_index ASC;
	`

	rows, err := l.db.QueryContext(ctx, query, runID, string(ApprovalPending))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var approvals []*Approval
	for rows.Next() {
		var app Approval
		var statusStr string

		err := rows.Scan(
			&app.ID,
			&app.RunID,
			&app.StepIndex,
			&app.Command,
			&app.Reason,
			&statusStr,
			&app.CreatedAt,
			&app.ResolvedAt,
		)
		if err != nil {
			return nil, err
		}

		app.Status = ApprovalStatus(statusStr)
		approvals = append(approvals, &app)
	}

	return approvals, rows.Err()
}

// GetAggregateStats returns cumulative token efficiency and dollar cost savings.
func (l *Ledger) GetAggregateStats(ctx context.Context) (*AggregateStats, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	query := `
	SELECT 
		COUNT(*) as total_runs,
		COALESCE(SUM(CASE WHEN status = 'running' OR status = 'waiting_approval' THEN 1 ELSE 0 END), 0) as active_runs,
		COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0) as completed_runs,
		COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) as failed_runs,
		COALESCE(SUM(tokens_in), 0) as total_tokens_in,
		COALESCE(SUM(tokens_out), 0) as total_tokens_out,
		COALESCE(SUM(tokens_saved), 0) as total_tokens_saved,
		COALESCE(SUM(cost_usd), 0.0) as total_cost_usd,
		COALESCE(SUM(cost_saved), 0.0) as total_cost_saved
	FROM runs;
	`

	row := l.db.QueryRowContext(ctx, query)
	var stats AggregateStats

	err := row.Scan(
		&stats.TotalRuns,
		&stats.ActiveRuns,
		&stats.CompletedRuns,
		&stats.FailedRuns,
		&stats.TotalTokensIn,
		&stats.TotalTokensOut,
		&stats.TotalTokensSaved,
		&stats.TotalCostUSD,
		&stats.TotalCostSaved,
	)
	if err != nil {
		return nil, err
	}

	totalTokens := (stats.TotalTokensIn + stats.TotalTokensOut) + stats.TotalTokensSaved
	if totalTokens > 0 {
		stats.AverageSavingsPct = (float64(stats.TotalTokensSaved) / float64(totalTokens)) * 100.0
	}

	return &stats, nil
}

// Close gracefully closes the SQLite connection.
func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.db.Close()
}
