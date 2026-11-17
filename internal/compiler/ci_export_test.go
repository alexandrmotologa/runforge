package compiler

import (
	"strings"
	"testing"
)

func TestCIExport(t *testing.T) {
	pb := &Playbook{
		Version:     1,
		Name:        "Automated PR Triage",
		Description: "Compiled deterministic test pipeline",
		Inputs: map[string]InputDefinition{
			"branch": {Type: "string", Default: "main", Description: "Target branch"},
		},
		Steps: []*PlaybookStep{
			{ID: "s1", Name: "Check Git Status", Run: "git status"},
			{ID: "s2", Name: "Run Tests", Run: "go test ./..."},
		},
		Metadata: PlaybookMetadata{
			EstimatedSavingsPct: 92.5,
		},
	}

	// 1. GitHub Actions
	gha, err := ExportCI(pb, "github-actions")
	if err != nil {
		t.Fatalf("failed to export GitHub Actions: %v", err)
	}
	if !strings.Contains(gha, "uses: actions/checkout@v4") || !strings.Contains(gha, "go test ./...") {
		t.Errorf("GitHub Actions workflow missing expected steps: %s", gha)
	}

	// 2. GitLab CI
	gitlab, err := ExportCI(pb, "gitlab")
	if err != nil {
		t.Fatalf("failed to export GitLab CI: %v", err)
	}
	if !strings.Contains(gitlab, "runforge_pipeline:") {
		t.Errorf("GitLab CI missing pipeline definition")
	}

	// 3. Dockerfile
	dockerfile, err := ExportCI(pb, "docker")
	if err != nil {
		t.Fatalf("failed to export Dockerfile: %v", err)
	}
	if !strings.Contains(dockerfile, "RUN go test ./...") {
		t.Errorf("Dockerfile missing test step execution")
	}
}
