package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alexandrmotologa/runforge/internal/compiler"
	"github.com/alexandrmotologa/runforge/internal/sandbox"
	"github.com/alexandrmotologa/runforge/internal/telemetry"
)

// HealingResult records an automated repair of a broken deterministic step.
type HealingResult struct {
	OriginalCommand string  `json:"original_command"`
	HealedCommand   string  `json:"healed_command"`
	Diagnosis       string  `json:"diagnosis"`
	TokensUsed      int64   `json:"tokens_used"`
	CostUSD         float64 `json:"cost_usd"`
	Success         bool    `json:"success"`
	Output          string  `json:"output"`
}

// DriftHealer diagnoses and repairs broken deterministic steps using low-cost SLMs.
type DriftHealer struct {
	client      ModelClient
	runner      sandbox.Runner
	costTracker *telemetry.CostTracker
	slmModel    string
}

func NewDriftHealer(client ModelClient, runner sandbox.Runner, slmModel string) *DriftHealer {
	if slmModel == "" {
		slmModel = "gemini-1.5-flash"
	}
	return &DriftHealer{
		client:      client,
		runner:      runner,
		costTracker: telemetry.NewCostTracker(),
		slmModel:    slmModel,
	}
}

// AttemptHeal analyzes failure output and applies an adaptive fix to the failing step.
func (h *DriftHealer) AttemptHeal(ctx context.Context, pbStep *compiler.PlaybookStep, exitCode int, errorOutput string, workingDir string) (*HealingResult, error) {
	prompt := fmt.Sprintf(`A deterministic workflow step failed in the execution sandbox.
Command: %s
Exit Code: %d
Error Output:
%s

Diagnose why this command failed (e.g. file was renamed, flag changed, missing directory) and provide the exact corrected command.
Respond strictly in JSON with keys "diagnosis" (one concise sentence) and "healed_command" (the exact executable string).`,
		pbStep.Run, exitCode, errorOutput)

	resp, err := h.client.Complete(ctx, &CompletionRequest{
		Model:       h.slmModel,
		Prompt:      prompt,
		Temperature: 0.1,
		MaxTokens:   300,
	})
	if err != nil {
		return nil, fmt.Errorf("model diagnosis failed: %w", err)
	}

	cost := h.costTracker.CalculateCost(h.slmModel, resp.TokensIn, resp.TokensOut)

	var repair struct {
		Diagnosis     string `json:"diagnosis"`
		HealedCommand string `json:"healed_command"`
	}

	cleanJSON := extractJSON(resp.Content)
	if err := json.Unmarshal([]byte(cleanJSON), &repair); err != nil || repair.HealedCommand == "" {
		// Fallback simple diagnosis if model returned raw text
		repair.Diagnosis = "Attempting fallback command retry"
		repair.HealedCommand = strings.TrimSpace(resp.Content)
	}

	// Verify the healed command in the sandbox
	parts := strings.Fields(repair.HealedCommand)
	if len(parts) == 0 {
		return &HealingResult{
			OriginalCommand: pbStep.Run,
			Diagnosis:       "No executable command generated",
			TokensUsed:      resp.TokensIn + resp.TokensOut,
			CostUSD:         cost,
			Success:         false,
		}, nil
	}

	testRes, err := h.runner.Execute(ctx, parts[0], parts[1:], sandbox.Limits{
		Timeout:    30 * time.Second,
		WorkingDir: workingDir,
	})

	if err == nil && testRes.ExitCode == 0 {
		// Step successfully healed! Update the playbook step in memory
		pbStep.Run = repair.HealedCommand
		return &HealingResult{
			OriginalCommand: pbStep.Run,
			HealedCommand:   repair.HealedCommand,
			Diagnosis:       repair.Diagnosis,
			TokensUsed:      resp.TokensIn + resp.TokensOut,
			CostUSD:         cost,
			Success:         true,
			Output:          testRes.Stdout,
		}, nil
	}

	return &HealingResult{
		OriginalCommand: pbStep.Run,
		HealedCommand:   repair.HealedCommand,
		Diagnosis:       repair.Diagnosis,
		TokensUsed:      resp.TokensIn + resp.TokensOut,
		CostUSD:         cost,
		Success:         false,
		Output:          fmt.Sprintf("Healed command still failed: %v", err),
	}, nil
}

func extractJSON(content string) string {
	re := regexp.MustCompile(`(?s)\{.*?\}`)
	match := re.FindString(content)
	if match != "" {
		return match
	}
	return content
}
