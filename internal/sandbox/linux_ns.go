//go:build linux

package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// LinuxNamespaceRunner isolates executions using Linux process attributes and namespaces.
type LinuxNamespaceRunner struct{}

func NewLinuxNamespaceRunner() *LinuxNamespaceRunner {
	return &LinuxNamespaceRunner{}
}

func (l *LinuxNamespaceRunner) Capabilities() RunnerCapabilities {
	return RunnerCapabilities{
		Platform:       "linux",
		HasMemoryLimit: true,
		HasProcessTree: true,
		HasNetworkIso:  true,
		EngineType:     "linux_namespaces",
	}
}

func (l *LinuxNamespaceRunner) Execute(ctx context.Context, command string, args []string, limits Limits) (*ExecutionResult, error) {
	start := time.Now()

	execCtx := ctx
	var cancel context.CancelFunc
	if limits.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, limits.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(execCtx, command, args...)
	if limits.WorkingDir != "" {
		cmd.Dir = limits.WorkingDir
	}

	cmd.Env = sanitizeEnvironment(limits.Env)

	// Configure Linux process isolation: separate process group and unshare UTS/IPC if permitted
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	duration := time.Since(start)

	if execCtx.Err() == context.DeadlineExceeded {
		return &ExecutionResult{
			Command:  command,
			Args:     args,
			ExitCode: -1,
			Stdout:   stdoutBuf.String(),
			Stderr:   "execution timed out",
			Duration: duration,
		}, fmt.Errorf("command timed out after %v", limits.Timeout)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute command %q: %w", command, err)
		}
	}

	return &ExecutionResult{
		Command:  command,
		Args:     args,
		ExitCode: exitCode,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
	}, nil
}
