package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// DirectRunner executes commands directly with environment filtering and strict timeouts.
type DirectRunner struct{}

func NewDirectRunner() *DirectRunner {
	return &DirectRunner{}
}

func (d *DirectRunner) Capabilities() RunnerCapabilities {
	return RunnerCapabilities{
		Platform:       runtime.GOOS,
		HasMemoryLimit: false,
		HasProcessTree: true,
		HasNetworkIso:  false,
		EngineType:     "direct_process",
	}
}

func (d *DirectRunner) Execute(ctx context.Context, command string, args []string, limits Limits) (*ExecutionResult, error) {
	start := time.Now()

	execCtx := ctx
	var cancel context.CancelFunc
	if limits.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, limits.Timeout)
		defer cancel()
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if strings.HasSuffix(strings.ToLower(command), ".cmd") || strings.HasSuffix(strings.ToLower(command), ".bat") || command == "cmd" {
			cmd = exec.CommandContext(execCtx, "cmd.exe", append([]string{"/c", command}, args...)...)
		} else {
			cmd = exec.CommandContext(execCtx, command, args...)
		}
	} else {
		cmd = exec.CommandContext(execCtx, command, args...)
	}

	if limits.WorkingDir != "" {
		cmd.Dir = limits.WorkingDir
	}

	cmd.Env = sanitizeEnvironment(limits.Env)

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
			Stderr:   "execution timed out: exceeded deadline",
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

// sanitizeEnvironment retains safe system environment variables while overlaying custom ones.
func sanitizeEnvironment(customEnv map[string]string) []string {
	safeKeys := map[string]bool{
		"PATH":        true,
		"PATHEXT":     true,
		"SYSTEMROOT":  true,
		"COMSPEC":     true,
		"TEMP":        true,
		"TMP":         true,
		"HOME":        true,
		"USERPROFILE": true,
		"LANG":        true,
		"LC_ALL":      true,
		"GO111MODULE": true,
		"GOPATH":      true,
		"GOROOT":      true,
	}

	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			upperKey := strings.ToUpper(parts[0])
			if safeKeys[upperKey] {
				envMap[parts[0]] = parts[1]
			}
		}
	}

	for k, v := range customEnv {
		envMap[k] = v
	}

	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}

	return result
}
