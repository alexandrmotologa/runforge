package engine

import (
	"strings"

	"github.com/alexandrmotologa/runforge/internal/durable"
)

// ExecutionStrategy specifies how a task step will be executed.
type ExecutionStrategy string

const (
	StrategyDeterministic ExecutionStrategy = "deterministic_sandbox"
	StrategySLM           ExecutionStrategy = "slm_semantic"
	StrategyFrontier      ExecutionStrategy = "frontier_explore"
)

// HandoffRouter determines the most cost-efficient execution strategy.
type HandoffRouter struct {
	FrontierModel string
	SLMModel      string
}

func NewHandoffRouter(frontierModel, slmModel string) *HandoffRouter {
	if frontierModel == "" {
		frontierModel = "claude-3-5-sonnet"
	}
	if slmModel == "" {
		slmModel = "gemini-1.5-flash"
	}
	return &HandoffRouter{
		FrontierModel: frontierModel,
		SLMModel:      slmModel,
	}
}

// SelectStrategy chooses the execution path based on run mode and step characteristics.
func (h *HandoffRouter) SelectStrategy(mode durable.RunMode, stepType string, command string) (ExecutionStrategy, string) {
	switch mode {
	case durable.ModeCompiled:
		// Compiled runs are 100% deterministic, 0 tokens
		return StrategyDeterministic, ""

	case durable.ModeHybrid:
		// Check if step requires natural language reasoning or rigid execution
		if isRigidCommand(stepType, command) {
			return StrategyDeterministic, ""
		}
		// Semantic tasks route to fast low-cost SLM
		return StrategySLM, h.SLMModel

	case durable.ModeExploratory:
		fallthrough
	default:
		// Initial zero-shot exploration defaults to frontier reasoning
		return StrategyFrontier, h.FrontierModel
	}
}

func isRigidCommand(stepType, command string) bool {
	t := strings.ToLower(stepType)
	c := strings.ToLower(command)

	if t == "command" || t == "write_file" || t == "read_file" {
		return true
	}

	rigidPrefixes := []string{"git ", "go ", "npm ", "pytest", "python ", "cat ", "ls ", "dir ", "mkdir ", "rm ", "cp ", "mv "}
	for _, p := range rigidPrefixes {
		if strings.HasPrefix(c, p) {
			return true
		}
	}

	return false
}
