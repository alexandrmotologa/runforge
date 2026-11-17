//go:build windows

package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WindowsJobRunner provides native process isolation and memory caps using Windows Job Objects.
type WindowsJobRunner struct{}

func NewWindowsJobRunner() *WindowsJobRunner {
	return &WindowsJobRunner{}
}

func (w *WindowsJobRunner) Capabilities() RunnerCapabilities {
	return RunnerCapabilities{
		Platform:       "windows",
		HasMemoryLimit: true,
		HasProcessTree: true,
		HasNetworkIso:  false,
		EngineType:     "windows_job_object",
	}
}

func (w *WindowsJobRunner) Execute(ctx context.Context, command string, args []string, limits Limits) (*ExecutionResult, error) {
	start := time.Now()

	// 1. Create a Windows Job Object
	jobHandle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		// Fallback to direct process execution if Job Object creation fails
		fallback := NewDirectRunner()
		return fallback.Execute(ctx, command, args, limits)
	}
	defer windows.CloseHandle(jobHandle)

	// 2. Configure Job Object limits
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	if limits.MaxMemoryBytes > 0 {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
		info.JobMemoryLimit = uintptr(limits.MaxMemoryBytes)
	}

	infoLen := uint32(unsafe.Sizeof(info))
	_, err = windows.SetInformationJobObject(
		jobHandle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		infoLen,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure job object limits: %w", err)
	}

	// 3. Prepare command
	execCtx := ctx
	var cancel context.CancelFunc
	if limits.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, limits.Timeout)
		defer cancel()
	}

	var cmd *exec.Cmd
	if strings.HasSuffix(strings.ToLower(command), ".cmd") || strings.HasSuffix(strings.ToLower(command), ".bat") || command == "cmd" {
		cmd = exec.CommandContext(execCtx, "cmd.exe", append([]string{"/c", command}, args...)...)
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

	// 4. Start child process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// 5. Open child process handle and assign to Job Object
	procHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_INFORMATION, false, uint32(cmd.Process.Pid))
	if err == nil {
		defer windows.CloseHandle(procHandle)
		_ = windows.AssignProcessToJobObject(jobHandle, procHandle)
	}

	// 6. Wait for process completion
	err = cmd.Wait()
	duration := time.Since(start)

	// 7. Query peak memory usage from Job Object
	var peakMemory int64
	var queryInfo windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	var returnedLen uint32
	qErr := windows.QueryInformationJobObject(
		jobHandle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&queryInfo)),
		uint32(unsafe.Sizeof(queryInfo)),
		&returnedLen,
	)
	if qErr == nil {
		peakMemory = int64(queryInfo.PeakJobMemoryUsed)
	}

	if execCtx.Err() == context.DeadlineExceeded {
		return &ExecutionResult{
			Command:         command,
			Args:            args,
			ExitCode:        -1,
			Stdout:          stdoutBuf.String(),
			Stderr:          "execution timed out: process tree terminated by Windows Job Object",
			Duration:        duration,
			MemoryPeakBytes: peakMemory,
		}, fmt.Errorf("command timed out after %v", limits.Timeout)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("process execution failed: %w", err)
		}
	}

	return &ExecutionResult{
		Command:         command,
		Args:            args,
		ExitCode:        exitCode,
		Stdout:          stdoutBuf.String(),
		Stderr:          stderrBuf.String(),
		Duration:        duration,
		MemoryPeakBytes: peakMemory,
	}, nil
}
