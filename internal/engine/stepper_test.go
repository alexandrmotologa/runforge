package engine

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/alexandrmotologa/runforge/internal/compiler"
	"github.com/alexandrmotologa/runforge/internal/durable"
	"github.com/alexandrmotologa/runforge/internal/sandbox"
)

func TestStepperLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "stepper_test.db")

	ledger, err := durable.NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}
	defer ledger.Close()

	runner := sandbox.NewDefaultRunner()
	pb := &compiler.Playbook{
		Version:     1,
		Name:        "stepper-test-wf",
		Description: "Interactive step debugger test",
		Steps: []*compiler.PlaybookStep{
			{ID: "s1", Name: "Step 1", Run: "cmd /c echo step_one"},
			{ID: "s2", Name: "Step 2", Run: "cmd /c echo step_two"},
		},
	}

	stepper, err := NewStepper(ledger, runner, pb, "run-stepper-01", map[string]interface{}{"env": "test"})
	if err != nil {
		t.Fatalf("failed to create stepper: %v", err)
	}

	ctx := context.Background()

	// 1. Step Next (runs step 0)
	step0, err := stepper.StepNext(ctx)
	if err != nil {
		t.Fatalf("step next failed: %v", err)
	}
	if step0.Status != durable.StepSuccess {
		t.Errorf("step 0 status not success: %s", step0.Status)
	}

	status := stepper.Status()
	if status.CurrentStepIndex != 1 {
		t.Errorf("expected current step index 1, got %d", status.CurrentStepIndex)
	}

	// 2. Set variable
	stepper.SetVariable("custom_tag", "v1.2.3")
	if stepper.Status().Variables["custom_tag"] != "v1.2.3" {
		t.Errorf("variable override not reflected in status")
	}

	// 3. Rewind to Step 0
	if err := stepper.Rewind(ctx, 0); err != nil {
		t.Fatalf("rewind failed: %v", err)
	}
	if stepper.Status().CurrentStepIndex != 0 {
		t.Errorf("expected rewind to step 0, got %d", stepper.Status().CurrentStepIndex)
	}

	// 4. Step again from 0
	step0Again, err := stepper.StepNext(ctx)
	if err != nil || step0Again.Status != durable.StepSuccess {
		t.Fatalf("step again failed: %v", err)
	}
}
