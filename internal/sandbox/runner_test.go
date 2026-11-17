package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSandboxExecution(t *testing.T) {
	runner := NewDefaultRunner()
	caps := runner.Capabilities()

	t.Logf("Testing runner on platform %s with engine %s", caps.Platform, caps.EngineType)

	ctx := context.Background()

	// 1. Basic command execution
	var cmd string
	var args []string
	if caps.Platform == "windows" {
		cmd = "cmd"
		args = []string{"/c", "echo", "runforge_sandbox_ok"}
	} else {
		cmd = "echo"
		args = []string{"runforge_sandbox_ok"}
	}

	result, err := runner.Execute(ctx, cmd, args, Limits{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "runforge_sandbox_ok") {
		t.Errorf("stdout missing expected token: got %q", result.Stdout)
	}

	// 2. Timeout enforcement test
	var timeoutCmd string
	var timeoutArgs []string
	if caps.Platform == "windows" {
		timeoutCmd = "powershell"
		timeoutArgs = []string{"-Command", "Start-Sleep -Milliseconds 1500"}
	} else {
		timeoutCmd = "sleep"
		timeoutArgs = []string{"2"}
	}

	res, err := runner.Execute(ctx, timeoutCmd, timeoutArgs, Limits{
		Timeout: 300 * time.Millisecond,
	})
	if err == nil {
		t.Errorf("expected timeout error, but command succeeded: res=%+v", res)
	}
}
