package compiler

import (
	"strings"
	"testing"
)

func TestCompilerPipeline(t *testing.T) {
	rawJSON := `{
		"id": "trace-code-review-01",
		"goal": "Review PR #42 on branch feat/login-security and verify tests pass",
		"steps": [
			{
				"index": 0,
				"tool": "bash",
				"command": "git status",
				"exit_code": 0,
				"tokens_in": 1200,
				"tokens_out": 150
			},
			{
				"index": 1,
				"tool": "read_file",
				"path": "README.md",
				"exit_code": 0,
				"tokens_in": 3500,
				"tokens_out": 200
			},
			{
				"index": 2,
				"tool": "read_file",
				"path": "README.md",
				"exit_code": 0,
				"tokens_in": 3500,
				"tokens_out": 200
			},
			{
				"index": 3,
				"tool": "bash",
				"command": "cat non_existent_config.json",
				"exit_code": 1,
				"tokens_in": 1500,
				"tokens_out": 100
			},
			{
				"index": 4,
				"tool": "bash",
				"command": "git checkout feat/login-security",
				"exit_code": 0,
				"tokens_in": 1800,
				"tokens_out": 120
			},
			{
				"index": 5,
				"tool": "bash",
				"command": "go test ./...",
				"exit_code": 0,
				"tokens_in": 4500,
				"tokens_out": 300
			}
		]
	}`

	// 1. Ingestion
	trace, err := ParseAutoFormat([]byte(rawJSON))
	if err != nil {
		t.Fatalf("failed to parse trajectory: %v", err)
	}
	if len(trace.Steps) != 6 {
		t.Fatalf("expected 6 steps, got %d", len(trace.Steps))
	}

	// 2. Causal graph & Pruning
	report, err := BuildCausalGraphAndPrune(trace)
	if err != nil {
		t.Fatalf("pruning failed: %v", err)
	}

	// Step 2 is duplicate read of README.md -> should be pruned
	// Step 3 is failed probe (cat non_existent_config.json) -> should be pruned
	if report.PrunedStepCount < 2 {
		t.Errorf("expected at least 2 pruned steps, got %d", report.PrunedStepCount)
	}
	t.Logf("Pruning reduction: %.1f%% (Pruned: %d, Retained: %d)", report.ReductionPercent, report.PrunedStepCount, report.RetainedStepCount)

	// 3. Playbook Compilation
	playbook, err := CompilePlaybook(trace, report)
	if err != nil {
		t.Fatalf("failed to compile playbook: %v", err)
	}

	// Verify parameterized inputs were discovered
	if _, ok := playbook.Inputs["branch_name"]; !ok {
		t.Errorf("expected 'branch_name' in extracted inputs, got %+v", playbook.Inputs)
	}

	yamlBytes, err := ToYAML(playbook)
	if err != nil {
		t.Fatalf("failed to serialize yaml: %v", err)
	}
	if !strings.Contains(string(yamlBytes), "${{ inputs.branch_name }}") {
		t.Errorf("expected parameterized branch placeholder in playbook yaml: %s", string(yamlBytes))
	}

	// 4. Standalone Script Codegen
	bashScript, err := GenerateBash(playbook)
	if err != nil {
		t.Fatalf("failed to generate bash: %v", err)
	}
	if !strings.Contains(bashScript, "set -euo pipefail") {
		t.Errorf("invalid bash script header")
	}

	pyScript, err := GeneratePython(playbook)
	if err != nil {
		t.Fatalf("failed to generate python: %v", err)
	}
	if !strings.Contains(pyScript, "def main():") {
		t.Errorf("invalid python script structure")
	}
}
