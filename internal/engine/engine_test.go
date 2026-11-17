package engine

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/alexandrmotologa/runforge/internal/compiler"
	"github.com/alexandrmotologa/runforge/internal/durable"
	"github.com/alexandrmotologa/runforge/internal/sandbox"
	"github.com/alexandrmotologa/runforge/internal/telemetry"
)

func TestOrchestratorExecution(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_engine.db")

	ledger, err := durable.NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}
	defer ledger.Close()

	runner := sandbox.NewDefaultRunner()
	client := NewMockClient()
	orchestrator := NewOrchestrator(ledger, runner, client)

	// Subscribe to events
	eventsCh := orchestrator.Subscribe()
	defer orchestrator.Unsubscribe(eventsCh)

	ctx := context.Background()

	// 1. Test deterministic playbook execution
	pb := &compiler.Playbook{
		Version:     1,
		Name:        "test-deterministic-workflow",
		Description: "Verify orchestrator executes steps cleanly into SQLite WAL",
		Steps: []*compiler.PlaybookStep{
			{
				ID:   "s1",
				Name: "Echo Hello",
				Type: "command",
				Run:  "cmd /c echo hello_runforge",
			},
			{
				ID:   "s2",
				Name: "Echo Done",
				Type: "command",
				Run:  "cmd /c echo finished_step",
			},
		},
		Metadata: compiler.PlaybookMetadata{
			OriginalTokens: 15000,
		},
	}

	run, err := orchestrator.RunPlaybook(ctx, pb, "run-engine-01", nil)
	if err != nil {
		t.Fatalf("playbook execution failed: %v", err)
	}

	if run.Status != durable.StatusCompleted {
		t.Errorf("expected status completed, got %s", run.Status)
	}

	if run.TokensSaved != 15000 {
		t.Errorf("expected 15000 tokens saved, got %d", run.TokensSaved)
	}

	// Verify steps in database
	steps, err := ledger.GetStepsByRun(ctx, run.ID)
	if err != nil || len(steps) != 2 {
		t.Fatalf("expected 2 recorded steps, got %d (err: %v)", len(steps), err)
	}

	for _, st := range steps {
		if st.Status != durable.StepSuccess {
			t.Errorf("step %s did not succeed: %s", st.Name, st.OutputPayload)
		}
	}

	// 2. Test Cost Tracker
	tracker := telemetry.NewCostTracker()
	costFrontier := tracker.CalculateCost("claude-3-5-sonnet", 100000, 20000)
	// (100k/1M)*3 + (20k/1M)*15 = 0.30 + 0.30 = $0.60
	if costFrontier < 0.59 || costFrontier > 0.61 {
		t.Errorf("expected ~$0.60 cost for frontier model, got %.4f", costFrontier)
	}

	savings := tracker.CalculateSavings("claude-3-5-sonnet", 100000, 20000, 0.0)
	if savings < 0.59 || savings > 0.61 {
		t.Errorf("expected ~$0.60 savings, got %.4f", savings)
	}
}
