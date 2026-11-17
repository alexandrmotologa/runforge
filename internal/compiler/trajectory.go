package compiler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
)

// TrajectoryStep represents a discrete turn, tool call, or observation from an agent execution.
type TrajectoryStep struct {
	Index        int                    `json:"index"`
	Type         string                 `json:"type"` // thought, action, observation, tool
	Tool         string                 `json:"tool,omitempty"`
	Command      string                 `json:"command,omitempty"`
	Path         string                 `json:"path,omitempty"`
	Content      string                 `json:"content,omitempty"`
	Input        map[string]interface{} `json:"input,omitempty"`
	Output       string                 `json:"output,omitempty"`
	ExitCode     int                    `json:"exit_code"`
	Error        string                 `json:"error,omitempty"`
	TokensIn     int64                  `json:"tokens_in,omitempty"`
	TokensOut    int64                  `json:"tokens_out,omitempty"`
	DurationMs   int64                  `json:"duration_ms,omitempty"`
	IsPruned     bool                   `json:"is_pruned,omitempty"`
	PruneReason  string                 `json:"prune_reason,omitempty"`
}

// TrajectoryTrace represents an entire recorded agent exploration session.
type TrajectoryTrace struct {
	ID             string            `json:"id"`
	SessionID      string            `json:"session_id,omitempty"`
	AgentName      string            `json:"agent_name,omitempty"`
	Goal           string            `json:"goal"`
	Model          string            `json:"model,omitempty"`
	Steps          []*TrajectoryStep `json:"steps"`
	FinalOutcome   string            `json:"final_outcome,omitempty"`
	TotalTokensIn  int64             `json:"total_tokens_in,omitempty"`
	TotalTokensOut int64             `json:"total_tokens_out,omitempty"`
	TotalCostUSD   float64           `json:"total_cost_usd,omitempty"`
}

// ParseAutoFormat intelligently detects and ingests standard JSON traces or Claude Code JSONL transcripts.
func ParseAutoFormat(data []byte) (*TrajectoryTrace, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty trajectory data")
	}

	// If starts with '{' or '[', attempt standard JSON parser
	if trimmed[0] == '{' || trimmed[0] == '[' {
		trace, err := ParseJSONTrajectory(trimmed)
		if err == nil && len(trace.Steps) > 0 {
			return trace, nil
		}
	}

	// Attempt JSONL transcript format
	return ParseTranscriptJSONL(trimmed)
}

// ParseJSONTrajectory ingests standard runforge or ReAct JSON formatted traces.
func ParseJSONTrajectory(data []byte) (*TrajectoryTrace, error) {
	// 1. Direct TrajectoryTrace structure
	var trace TrajectoryTrace
	if err := json.Unmarshal(data, &trace); err == nil && len(trace.Steps) > 0 {
		normalizeSteps(&trace)
		return &trace, nil
	}

	// 2. Array of steps
	var stepList []*TrajectoryStep
	if err := json.Unmarshal(data, &stepList); err == nil && len(stepList) > 0 {
		t := &TrajectoryTrace{
			ID:    "trace-imported",
			Goal:  "Imported agent trace",
			Steps: stepList,
		}
		normalizeSteps(t)
		return t, nil
	}

	// 3. Generic map wrapper (e.g. {"steps": [...]})
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err == nil {
		t := &TrajectoryTrace{
			ID: "trace-imported",
		}
		if goal, ok := rawMap["goal"].(string); ok {
			t.Goal = goal
		}
		if model, ok := rawMap["model"].(string); ok {
			t.Model = model
		}

		if rawSteps, ok := rawMap["steps"].([]interface{}); ok {
			for i, r := range rawSteps {
				stepBytes, _ := json.Marshal(r)
				var st TrajectoryStep
				if err := json.Unmarshal(stepBytes, &st); err == nil {
					st.Index = i
					t.Steps = append(t.Steps, &st)
				}
			}
			normalizeSteps(t)
			return t, nil
		}
	}

	return nil, fmt.Errorf("unrecognized JSON trajectory schema")
}

// ParseTranscriptJSONL ingests step-by-step JSONL logs (e.g. Claude Code or transcript.jsonl).
func ParseTranscriptJSONL(data []byte) (*TrajectoryTrace, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	trace := &TrajectoryTrace{
		ID:    "transcript-trace",
		Goal:  "Extracted from JSONL transcript",
		Steps: make([]*TrajectoryStep, 0),
	}

	stepIdx := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var row map[string]interface{}
		if err := json.Unmarshal(line, &row); err != nil {
			continue
		}

		step := &TrajectoryStep{
			Index: stepIdx,
			Type:  "action",
		}

		if stepType, ok := row["type"].(string); ok {
			step.Type = stepType
		}

		// Tool calls array
		if toolCalls, ok := row["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
			for _, tcRaw := range toolCalls {
				if tc, ok := tcRaw.(map[string]interface{}); ok {
					if fnName, ok := tc["name"].(string); ok {
						step.Tool = fnName
					}
					if args, ok := tc["args"].(map[string]interface{}); ok {
						step.Input = args
						if cmd, ok := args["CommandLine"].(string); ok {
							step.Command = cmd
						} else if cmd, ok := args["command"].(string); ok {
							step.Command = cmd
						}
						if path, ok := args["Path"].(string); ok {
							step.Path = path
						} else if path, ok := args["path"].(string); ok {
							step.Path = path
						}
					}
				}
			}
		}

		// Direct content or output
		if content, ok := row["content"].(string); ok {
			step.Output = content
		}

		trace.Steps = append(trace.Steps, step)
		stepIdx++
	}

	if len(trace.Steps) == 0 {
		return nil, fmt.Errorf("no valid steps found in JSONL transcript")
	}

	normalizeSteps(trace)
	return trace, nil
}

func normalizeSteps(trace *TrajectoryTrace) {
	for i, step := range trace.Steps {
		step.Index = i
		// Extract command if in Input
		if step.Command == "" && step.Input != nil {
			if cmd, ok := step.Input["command"].(string); ok {
				step.Command = cmd
			} else if cmd, ok := step.Input["CommandLine"].(string); ok {
				step.Command = cmd
			}
		}
		// Extract path if in Input
		if step.Path == "" && step.Input != nil {
			if p, ok := step.Input["path"].(string); ok {
				step.Path = p
			} else if p, ok := step.Input["Path"].(string); ok {
				step.Path = p
			}
		}
		// If tool is empty, detect tool from command or type
		if step.Tool == "" {
			if step.Command != "" {
				step.Tool = "bash"
			} else if step.Path != "" {
				if step.Content != "" {
					step.Tool = "write_file"
				} else {
					step.Tool = "read_file"
				}
			} else {
				step.Tool = "generic"
			}
		}

		if step.TokensIn > 0 {
			trace.TotalTokensIn += step.TokensIn
		}
		if step.TokensOut > 0 {
			trace.TotalTokensOut += step.TokensOut
		}
	}

	if trace.TotalCostUSD == 0 && (trace.TotalTokensIn > 0 || trace.TotalTokensOut > 0) {
		// Default estimation based on frontier model ($3/M in, $15/M out)
		trace.TotalCostUSD = (float64(trace.TotalTokensIn)*3.0 + float64(trace.TotalTokensOut)*15.0) / 1000000.0
	}
}
