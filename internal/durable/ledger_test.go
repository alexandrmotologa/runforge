package durable

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLedgerLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_ledger.db")

	ledger, err := NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}
	defer ledger.Close()

	ctx := context.Background()

	// 1. Create Run
	run := &Run{
		ID:         "run-test-01",
		WorkflowID: "wf-review-pr",
		Name:       "Test PR Review Run",
		Status:     StatusRunning,
		Mode:       ModeExploratory,
		Model:      "claude-3-5-sonnet",
		Prompt:     "Inspect git diff and check for security bugs",
		InputVars: map[string]interface{}{
			"repo":  "github.com/alexandrmotologa/runforge",
			"pr_id": 42,
		},
		TokensIn:  12500,
		TokensOut: 1800,
		CostUSD:   0.065,
	}

	if err := ledger.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// 2. Fetch Run
	fetched, err := ledger.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if fetched.Name != run.Name || fetched.WorkflowID != run.WorkflowID {
		t.Errorf("mismatched run attributes: got %+v, want %+v", fetched, run)
	}

	// 3. Record Steps
	step1 := &Step{
		ID:            "step-01",
		RunID:         run.ID,
		StepIndex:     0,
		Name:          "List files",
		Type:          StepTypeTool,
		ToolName:      "bash",
		InputPayload:  `{"command": "git status --porcelain"}`,
		OutputPayload: "M internal/durable/ledger.go",
		ExitCode:      0,
		DurationMs:    12,
		Status:        StepSuccess,
	}
	if err := ledger.RecordStep(ctx, step1); err != nil {
		t.Fatalf("failed to record step 1: %v", err)
	}

	step2 := &Step{
		ID:           "step-02",
		RunID:        run.ID,
		StepIndex:    1,
		Name:         "Run unit tests",
		Type:         StepTypeCommand,
		ToolName:     "bash",
		InputPayload: `{"command": "go test ./..."}`,
		ExitCode:     0,
		DurationMs:   850,
		Status:       StepRunning,
	}
	if err := ledger.RecordStep(ctx, step2); err != nil {
		t.Fatalf("failed to record step 2: %v", err)
	}

	// Update Step 2 to success
	step2.Status = StepSuccess
	step2.OutputPayload = "PASS: all tests passed"
	if err := ledger.UpdateStep(ctx, step2); err != nil {
		t.Fatalf("failed to update step 2: %v", err)
	}

	steps, err := ledger.GetStepsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("failed to get steps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	// 4. Create Checkpoint
	cp := &Checkpoint{
		ID:        "cp-01",
		RunID:     run.ID,
		StepIndex: 1,
		StateData: `{"last_analyzed_commit": "abc1234"}`,
		Variables: map[string]interface{}{
			"test_status": "clean",
		},
	}
	if err := ledger.CreateCheckpoint(ctx, cp); err != nil {
		t.Fatalf("failed to create checkpoint: %v", err)
	}

	latestCp, err := ledger.GetLatestCheckpoint(ctx, run.ID)
	if err != nil {
		t.Fatalf("failed to get latest checkpoint: %v", err)
	}
	if latestCp.StepIndex != 1 {
		t.Errorf("expected step index 1, got %d", latestCp.StepIndex)
	}

	// 5. Test Recovery and Interruption detection
	interrupted, err := ledger.FindInterruptedRuns(ctx)
	if err != nil {
		t.Fatalf("failed to find interrupted runs: %v", err)
	}
	if len(interrupted) != 1 {
		t.Fatalf("expected 1 interrupted run, got %d", len(interrupted))
	}

	resumePlan, err := ledger.GetResumePlan(ctx, run.ID)
	if err != nil {
		t.Fatalf("failed to build resume plan: %v", err)
	}
	if resumePlan.NextStepIndex != 2 {
		t.Errorf("expected next step index 2, got %d", resumePlan.NextStepIndex)
	}

	// 6. Test Approval Gates
	app := &Approval{
		ID:        "app-01",
		RunID:     run.ID,
		StepIndex: 2,
		Command:   "git push origin main",
		Reason:    "Destructive mutating operation to remote repository",
		Status:    ApprovalPending,
	}
	if err := ledger.RecordApproval(ctx, app); err != nil {
		t.Fatalf("failed to record approval: %v", err)
	}

	pendingApps, err := ledger.GetPendingApprovals(ctx, run.ID)
	if err != nil || len(pendingApps) != 1 {
		t.Fatalf("expected 1 pending approval, got %d (err: %v)", len(pendingApps), err)
	}

	if err := ledger.UpdateApproval(ctx, app.ID, ApprovalApproved); err != nil {
		t.Fatalf("failed to update approval: %v", err)
	}

	resolvedApp, err := ledger.GetApproval(ctx, app.ID)
	if err != nil || resolvedApp.Status != ApprovalApproved {
		t.Fatalf("approval not marked approved: %+v", resolvedApp)
	}

	// 7. Complete Run and check stats
	if err := ledger.UpdateRunStatus(ctx, run.ID, StatusCompleted, 12500, 1800, 50000, 0.065, 0.585); err != nil {
		t.Fatalf("failed to complete run: %v", err)
	}

	stats, err := ledger.GetAggregateStats(ctx)
	if err != nil {
		t.Fatalf("failed to get aggregate stats: %v", err)
	}
	if stats.CompletedRuns != 1 || stats.TotalTokensSaved != 50000 {
		t.Errorf("unexpected aggregate stats: %+v", stats)
	}
}
