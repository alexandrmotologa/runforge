package compiler

import (
	"fmt"
	"regexp"
	"strings"
)

// InputDefinition defines a parameterized input variable in a compiled playbook.
type InputDefinition struct {
	Type        string `json:"type" yaml:"type"`
	Default     string `json:"default" yaml:"default"`
	Description string `json:"description" yaml:"description"`
}

// ParameterizationResult maps extracted variable names to input definitions and transformed steps.
type ParameterizationResult struct {
	Inputs       map[string]InputDefinition
	Replacements map[string]string
}

// ExtractParameters discovers static values suitable for parameterization in reusable playbooks.
func ExtractParameters(steps []*TrajectoryStep) *ParameterizationResult {
	result := &ParameterizationResult{
		Inputs:       make(map[string]InputDefinition),
		Replacements: make(map[string]string),
	}

	// 1. Detect Git branch names (e.g. feat/..., fix/..., bugfix/...)
	branchRegex := regexp.MustCompile(`(feat|fix|chore|refactor)/[a-zA-Z0-9_\-]+`)
	for _, step := range steps {
		if step.IsPruned {
			continue
		}
		if match := branchRegex.FindString(step.Command); match != "" {
			varName := "branch_name"
			result.Inputs[varName] = InputDefinition{
				Type:        "string",
				Default:     match,
				Description: "Target Git branch name",
			}
			result.Replacements[match] = fmt.Sprintf("${{ inputs.%s }}", varName)
			break
		}
	}

	// 2. Detect PR numbers or Issue IDs (e.g. #123, PR 456, pr_id: 42)
	prRegex := regexp.MustCompile(`\b(pr|issue)[_\-\s]?(\d{1,6})\b`)
	for _, step := range steps {
		if step.IsPruned {
			continue
		}
		if match := prRegex.FindStringSubmatch(strings.ToLower(step.Command)); len(match) == 3 {
			varName := fmt.Sprintf("%s_id", match[1])
			result.Inputs[varName] = InputDefinition{
				Type:        "string",
				Default:     match[2],
				Description: fmt.Sprintf("Target %s identifier", strings.ToUpper(match[1])),
			}
			result.Replacements[match[0]] = fmt.Sprintf("${{ inputs.%s }}", varName)
			break
		}
	}

	return result
}

// ApplyParameters replaces extracted literal values with ${{ inputs.key }} syntax.
func ApplyParameters(text string, replacements map[string]string) string {
	out := text
	for literal, placeholder := range replacements {
		out = strings.ReplaceAll(out, literal, placeholder)
	}
	return out
}
