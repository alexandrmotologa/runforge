package engine

import (
	"context"
	"testing"

	"github.com/alexandrmotologa/runforge/internal/compiler"
	"github.com/alexandrmotologa/runforge/internal/sandbox"
)

type healTestMockClient struct{}

func (h *healTestMockClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Synthesize an adaptive command fix in JSON
	return &CompletionResponse{
		Content:   `{"diagnosis": "Missing output directory", "healed_command": "cmd /c echo healed_successfully"}`,
		TokensIn:  80,
		TokensOut: 25,
		Model:     req.Model,
	}, nil
}

func TestDriftHealer(t *testing.T) {
	runner := sandbox.NewDefaultRunner()
	mockClient := &healTestMockClient{}
	healer := NewDriftHealer(mockClient, runner, "gemini-1.5-flash")

	pbStep := &compiler.PlaybookStep{
		ID:   "step-fail-01",
		Name: "Failing step",
		Run:  "cmd /c exit 1",
	}

	res, err := healer.AttemptHeal(context.Background(), pbStep, 1, "Exit code 1 occurred", "")
	if err != nil {
		t.Fatalf("healing attempt errored: %v", err)
	}

	if !res.Success {
		t.Fatalf("expected step to be healed, got failure: %+v", res)
	}

	if pbStep.Run != "cmd /c echo healed_successfully" {
		t.Errorf("step command not updated to healed version: %s", pbStep.Run)
	}

	t.Logf("Step successfully healed! Diagnosis: %s, New command: %s", res.Diagnosis, res.HealedCommand)
}
