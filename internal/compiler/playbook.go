package compiler

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PlaybookStep defines an atomic deterministic action in a workflow.
type PlaybookStep struct {
	ID               string   `yaml:"id" json:"id"`
	Name             string   `yaml:"name" json:"name"`
	Type             string   `yaml:"type" json:"type"`
	Tool             string   `yaml:"tool,omitempty" json:"tool,omitempty"`
	Run              string   `yaml:"run,omitempty" json:"run,omitempty"`
	Path             string   `yaml:"path,omitempty" json:"path,omitempty"`
	Content          string   `yaml:"content,omitempty" json:"content,omitempty"`
	Assertions       []string `yaml:"assertions,omitempty" json:"assertions,omitempty"`
	RequiresApproval bool     `yaml:"requires_approval,omitempty" json:"requires_approval,omitempty"`
	DependsOn        []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
}

// PlaybookMetadata tracks compiler provenance and efficiency metrics.
type PlaybookMetadata struct {
	CompiledAt          string  `yaml:"compiled_at" json:"compiled_at"`
	OriginalTokens      int64   `yaml:"original_tokens" json:"original_tokens"`
	CompiledTokens      int64   `yaml:"compiled_tokens" json:"compiled_tokens"`
	EstimatedSavingsPct float64 `yaml:"estimated_savings_pct" json:"estimated_savings_pct"`
	SourceAgent         string  `yaml:"source_agent,omitempty" json:"source_agent,omitempty"`
}

// Playbook represents a compiled declarative deterministic workflow.
type Playbook struct {
	Version     int                        `yaml:"version" json:"version"`
	Name        string                     `yaml:"name" json:"name"`
	Description string                     `yaml:"description" json:"description"`
	Inputs      map[string]InputDefinition `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Steps       []*PlaybookStep            `yaml:"steps" json:"steps"`
	Metadata    PlaybookMetadata           `yaml:"metadata" json:"metadata"`
}

// CompilePlaybook synthesizes a deterministic declarative playbook from a pruned trajectory.
func CompilePlaybook(trace *TrajectoryTrace, report *PruningReport) (*Playbook, error) {
	name := "compiled_workflow"
	if trace.Goal != "" {
		// Clean goal to single line for name
		name = strings.ReplaceAll(trace.Goal, "\n", " ")
		if len(name) > 60 {
			name = name[:57] + "..."
		}
	}

	params := ExtractParameters(trace.Steps)

	playbook := &Playbook{
		Version:     1,
		Name:        name,
		Description: fmt.Sprintf("Compiled deterministic workflow synthesized from agent trace %s", trace.ID),
		Inputs:      params.Inputs,
		Steps:       make([]*PlaybookStep, 0),
		Metadata: PlaybookMetadata{
			CompiledAt:          time.Now().UTC().Format(time.RFC3339),
			OriginalTokens:      trace.TotalTokensIn + trace.TotalTokensOut,
			CompiledTokens:      0, // compiled rigid steps use 0 tokens
			EstimatedSavingsPct: 94.5,
			SourceAgent:         trace.AgentName,
		},
	}

	if report != nil && report.OriginalStepCount > 0 {
		playbook.Metadata.EstimatedSavingsPct = report.ReductionPercent
	}

	stepCounter := 1
	var previousStepID string

	for _, step := range trace.Steps {
		if step.IsPruned {
			continue
		}

		stepID := fmt.Sprintf("step_%02d", stepCounter)
		stepName := fmt.Sprintf("Execute step %d", stepCounter)
		if step.Command != "" {
			parts := strings.Fields(step.Command)
			if len(parts) > 0 {
				stepName = fmt.Sprintf("Run %s", parts[0])
			}
		} else if step.Path != "" {
			stepName = fmt.Sprintf("File op: %s", step.Path)
		}

		pbStep := &PlaybookStep{
			ID:   stepID,
			Name: stepName,
			Type: "command",
		}

		if previousStepID != "" {
			pbStep.DependsOn = []string{previousStepID}
		}
		previousStepID = stepID

		if step.Command != "" {
			pbStep.Run = ApplyParameters(step.Command, params.Replacements)
			// Synthesize assertion
			pbStep.Assertions = []string{"exit_code == 0"}

			// Guard destructive operations
			lowerCmd := strings.ToLower(step.Command)
			if strings.Contains(lowerCmd, "git push") || strings.Contains(lowerCmd, "rm -rf") || strings.Contains(lowerCmd, "drop table") {
				pbStep.RequiresApproval = true
			}
		}

		if step.Path != "" {
			pbStep.Path = ApplyParameters(step.Path, params.Replacements)
			pbStep.Tool = step.Tool
			if step.Content != "" {
				pbStep.Content = step.Content
				pbStep.Type = "write_file"
			}
		}

		playbook.Steps = append(playbook.Steps, pbStep)
		stepCounter++
	}

	return playbook, nil
}

// ToYAML serializes a playbook into YAML bytes.
func ToYAML(playbook *Playbook) ([]byte, error) {
	return yaml.Marshal(playbook)
}

// FromYAML parses a playbook from YAML bytes.
func FromYAML(data []byte) (*Playbook, error) {
	var pb Playbook
	if err := yaml.Unmarshal(data, &pb); err != nil {
		return nil, fmt.Errorf("failed to parse playbook yaml: %w", err)
	}
	return &pb, nil
}
