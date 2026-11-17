package compiler

import (
	"path/filepath"
	"strings"
)

// CausalNode represents a node in the causal dependency DAG.
type CausalNode struct {
	Step         *TrajectoryStep
	Index        int
	ReadsFiles   []string
	WritesFiles  []string
	ExecutesCmd  string
	IsMutating   bool
	IsValidation bool
	IsFailed     bool
	Dependencies []*CausalNode
	Dependents   []*CausalNode
	Reachable    bool
}

// PruningReport contains the result of the causal DAG reduction.
type PruningReport struct {
	OriginalStepCount int               `json:"original_step_count"`
	PrunedStepCount   int               `json:"pruned_step_count"`
	RetainedStepCount int               `json:"retained_step_count"`
	ReductionPercent  float64           `json:"reduction_percent"`
	EstimatedTokensIn int64             `json:"estimated_tokens_in"`
	EstimatedTokensSaved int64          `json:"estimated_tokens_saved"`
	PrunedSteps       []*TrajectoryStep `json:"pruned_steps"`
	RetainedSteps     []*TrajectoryStep `json:"retained_steps"`
}

// BuildCausalGraphAndPrune constructs a dependency graph from a trace and eliminates non-contributing branches.
func BuildCausalGraphAndPrune(trace *TrajectoryTrace) (*PruningReport, error) {
	nodes := make([]*CausalNode, len(trace.Steps))

	// Track last writer and reader of files to build causal edges
	fileLastWriter := make(map[string]*CausalNode)
	fileReaders := make(map[string][]*CausalNode)
	seenReads := make(map[string]int) // file path -> step index

	for i, step := range trace.Steps {
		node := &CausalNode{
			Step:        step,
			Index:       i,
			ExecutesCmd: step.Command,
			IsFailed:    step.ExitCode != 0 || step.Error != "",
		}

		classifyNode(node)
		nodes[i] = node

		// Record causal read/write relationships
		for _, wFile := range node.WritesFiles {
			fileLastWriter[wFile] = node
			// Invalidate seen reads for modified file
			delete(seenReads, wFile)
		}

		for _, rFile := range node.ReadsFiles {
			if writer, exists := fileLastWriter[rFile]; exists {
				node.Dependencies = append(node.Dependencies, writer)
				writer.Dependents = append(writer.Dependents, node)
			}
			fileReaders[rFile] = append(fileReaders[rFile], node)
		}
	}

	// 1. Identify Target Actions: Mutating actions and successful validation steps
	var targetNodes []*CausalNode
	for _, node := range nodes {
		if node.IsMutating || (node.IsValidation && !node.IsFailed) {
			targetNodes = append(targetNodes, node)
		}
	}

	// If no obvious mutations/validations, retain the final successful steps
	if len(targetNodes) == 0 && len(nodes) > 0 {
		for i := len(nodes) - 1; i >= 0 && len(targetNodes) < 3; i-- {
			if !nodes[i].IsFailed {
				targetNodes = append(targetNodes, nodes[i])
			}
		}
	}

	// 2. Mark Reachable nodes via backward traversal from targets
	for _, target := range targetNodes {
		markReachable(target)
	}

	// 3. Mark duplicate reads and failed probes as pruned
	for _, node := range nodes {
		st := node.Step
		cleanPath := filepath.Clean(st.Path)

		// A. Prune failed exploratory probe that did not contribute
		if node.IsFailed && !node.IsValidation {
			st.IsPruned = true
			st.PruneReason = "failed exploratory probe"
			continue
		}

		// B. Prune duplicate identical file reads if file was not modified
		if isReadAction(st.Tool, st.Command) && cleanPath != "." && cleanPath != "" {
			if prevIdx, seen := seenReads[cleanPath]; seen && prevIdx < node.Index {
				st.IsPruned = true
				st.PruneReason = "redundant duplicate file read"
				continue
			}
			seenReads[cleanPath] = node.Index
		}

		// C. Prune pure informational exploration if not reachable by any mutation or validation
		if !node.Reachable && !node.IsMutating && !node.IsValidation {
			st.IsPruned = true
			st.PruneReason = "dead-end exploratory branch"
			continue
		}

		// Keep step
		st.IsPruned = false
		st.PruneReason = ""
	}

	// Build summary report
	report := &PruningReport{
		OriginalStepCount: len(trace.Steps),
		PrunedSteps:       make([]*TrajectoryStep, 0),
		RetainedSteps:     make([]*TrajectoryStep, 0),
	}

	for _, step := range trace.Steps {
		if step.IsPruned {
			report.PrunedStepCount++
			report.PrunedSteps = append(report.PrunedSteps, step)
			report.EstimatedTokensSaved += step.TokensIn + step.TokensOut
		} else {
			report.RetainedStepCount++
			report.RetainedSteps = append(report.RetainedSteps, step)
			report.EstimatedTokensIn += step.TokensIn
		}
	}

	if report.OriginalStepCount > 0 {
		report.ReductionPercent = (float64(report.PrunedStepCount) / float64(report.OriginalStepCount)) * 100.0
	}

	// If no explicit token counts were recorded, estimate based on step count
	if report.EstimatedTokensSaved == 0 && report.PrunedStepCount > 0 {
		report.EstimatedTokensSaved = int64(report.PrunedStepCount) * 3500
	}

	return report, nil
}

func markReachable(node *CausalNode) {
	if node.Reachable {
		return
	}
	node.Reachable = true
	for _, dep := range node.Dependencies {
		markReachable(dep)
	}
}

func classifyNode(node *CausalNode) {
	st := node.Step
	tool := strings.ToLower(st.Tool)
	cmd := strings.ToLower(st.Command)

	// File read tool or command
	if tool == "read_file" || tool == "read" || tool == "cat" || strings.HasPrefix(cmd, "cat ") || strings.HasPrefix(cmd, "head ") || strings.HasPrefix(cmd, "tail ") {
		if st.Path != "" {
			node.ReadsFiles = append(node.ReadsFiles, filepath.Clean(st.Path))
		}
	}

	// File write tool or command
	if tool == "write_file" || tool == "edit_file" || tool == "write" || tool == "create_file" {
		node.IsMutating = true
		if st.Path != "" {
			node.WritesFiles = append(node.WritesFiles, filepath.Clean(st.Path))
		}
	}

	// Mutating shell commands
	if strings.Contains(cmd, "git commit") || strings.Contains(cmd, "git checkout") ||
		strings.Contains(cmd, "git switch") || strings.Contains(cmd, "git merge") ||
		strings.Contains(cmd, "git pull") || strings.Contains(cmd, "npm install") ||
		strings.Contains(cmd, "go get") || strings.Contains(cmd, "mkdir") ||
		strings.Contains(cmd, "rm ") || strings.Contains(cmd, "touch ") ||
		strings.Contains(cmd, "cp ") || strings.Contains(cmd, "mv ") ||
		strings.Contains(cmd, ">") {
		node.IsMutating = true
	}

	// Validation and test commands
	if strings.Contains(cmd, "go test") || strings.Contains(cmd, "npm test") ||
		strings.Contains(cmd, "pytest") || strings.Contains(cmd, "cargo test") ||
		strings.Contains(cmd, "git diff") || strings.Contains(cmd, "git status") ||
		strings.Contains(cmd, "golangci-lint") || strings.Contains(cmd, "eslint") {
		node.IsValidation = true
	}
}

func isReadAction(tool, cmd string) bool {
	t := strings.ToLower(tool)
	c := strings.ToLower(cmd)
	return t == "read_file" || t == "read" || strings.HasPrefix(c, "cat ") || strings.HasPrefix(c, "type ") || strings.HasPrefix(c, "ls ") || strings.HasPrefix(c, "dir")
}
