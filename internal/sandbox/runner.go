package sandbox

import (
	"context"
	"time"
)

// Limits specifies resource boundaries for an isolated execution.
type Limits struct {
	Timeout        time.Duration     `json:"timeout"`
	MaxMemoryBytes int64             `json:"max_memory_bytes"`
	MaxCPURate     int               `json:"max_cpu_rate"` // Percentage 1-100
	WorkingDir     string            `json:"working_dir"`
	Env            map[string]string `json:"env"`
}

// ExecutionResult captures stdout, stderr, and resource metrics.
type ExecutionResult struct {
	Command         string        `json:"command"`
	Args            []string      `json:"args"`
	ExitCode        int           `json:"exit_code"`
	Stdout          string        `json:"stdout"`
	Stderr          string        `json:"stderr"`
	Duration        time.Duration `json:"duration"`
	MemoryPeakBytes int64         `json:"memory_peak_bytes"`
}

// RunnerCapabilities declares the sandboxing features supported on this platform.
type RunnerCapabilities struct {
	Platform       string `json:"platform"`
	HasMemoryLimit bool   `json:"has_memory_limit"`
	HasProcessTree bool   `json:"has_process_tree"`
	HasNetworkIso  bool   `json:"has_network_iso"`
	EngineType     string `json:"engine_type"`
}

// Runner provides an isolated execution environment for untrusted or deterministic commands.
type Runner interface {
	Execute(ctx context.Context, command string, args []string, limits Limits) (*ExecutionResult, error)
	Capabilities() RunnerCapabilities
}
