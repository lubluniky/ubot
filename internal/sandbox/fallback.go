// Package sandbox provides a secure container-based execution environment.
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Default fallback configuration values.
const (
	DefaultFallbackTimeout = 30 * time.Second
	DefaultFallbackWorkDir = ""
	MaxFallbackOutputLen   = 1024 * 1024 // 1MB
)

// LocalExecutor provides command execution without Docker.
// It uses os/exec directly but still applies command guard checks.
// This is intended as a fallback when Docker is not available.
type LocalExecutor struct {
	// WorkDir is the working directory for command execution.
	// If empty, uses the current directory.
	WorkDir string

	// Timeout is the maximum duration for command execution.
	// Default: 30s
	Timeout time.Duration

	// AllowedShells specifies which shells can be used.
	// If empty, defaults to platform-appropriate shells.
	AllowedShells []string

	// MaxOutputLen is the maximum output length in bytes.
	// Default: 1MB
	MaxOutputLen int

	// Environment variables to add to commands.
	// These are added to the existing environment.
	Env []string
}

// NewLocalExecutor creates a new LocalExecutor with default settings.
func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{
		WorkDir:       DefaultFallbackWorkDir,
		Timeout:       DefaultFallbackTimeout,
		AllowedShells: defaultShells(),
		MaxOutputLen:  MaxFallbackOutputLen,
	}
}

// NewLocalExecutorWithConfig creates a new LocalExecutor with the given settings.
func NewLocalExecutorWithConfig(workDir string, timeout time.Duration) *LocalExecutor {
	if timeout <= 0 {
		timeout = DefaultFallbackTimeout
	}

	return &LocalExecutor{
		WorkDir:       workDir,
		Timeout:       timeout,
		AllowedShells: defaultShells(),
		MaxOutputLen:  MaxFallbackOutputLen,
	}
}

// defaultShells returns the default shells for the current platform.
func defaultShells() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "powershell"}
	}
	return []string{"sh", "bash", "zsh"}
}

// Execute runs a command and returns the output.
// The command guard is applied before execution.
func (e *LocalExecutor) Execute(ctx context.Context, cmd []string) (stdout, stderr string, exitCode int, err error) {
	if len(cmd) == 0 {
		return "", "", -1, errors.New("empty command")
	}

	// Build full command string for guard check
	fullCmd := strings.Join(cmd, " ")

	// Apply command guard
	if reason := GuardCommand(fullCmd); reason != "" {
		return "", "", -1, fmt.Errorf("command blocked: %s", reason)
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	// Create the command
	command := exec.CommandContext(execCtx, cmd[0], cmd[1:]...)

	// Set working directory if specified
	if e.WorkDir != "" {
		absWorkDir, err := filepath.Abs(e.WorkDir)
		if err != nil {
			return "", "", -1, fmt.Errorf("invalid working directory: %w", err)
		}
		if _, err := os.Stat(absWorkDir); err != nil {
			return "", "", -1, fmt.Errorf("working directory does not exist: %w", err)
		}
		command.Dir = absWorkDir
	}

	// Set environment
	if len(e.Env) > 0 {
		command.Env = append(os.Environ(), e.Env...)
	}

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	command.Stdout = &limitedWriter{w: &stdoutBuf, limit: e.MaxOutputLen}
	command.Stderr = &limitedWriter{w: &stderrBuf, limit: e.MaxOutputLen}

	// Run the command
	err = command.Run()

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	// Check for timeout
	if execCtx.Err() == context.DeadlineExceeded {
		return stdout, stderr, -1, fmt.Errorf("command timed out after %v", e.Timeout)
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return stdout, stderr, -1, fmt.Errorf("command cancelled: %w", ctx.Err())
	}

	// Get exit code
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout, stderr, exitErr.ExitCode(), nil
		}
		return stdout, stderr, -1, fmt.Errorf("command execution failed: %w", err)
	}

	return stdout, stderr, 0, nil
}

// ExecuteShell runs a shell command and returns the output.
// The command is wrapped in a shell invocation.
func (e *LocalExecutor) ExecuteShell(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	// Find an available shell
	shell := e.findShell()
	if shell == "" {
		return "", "", -1, errors.New("no suitable shell found")
	}

	// Build shell command based on platform
	var cmd []string
	if runtime.GOOS == "windows" {
		if strings.Contains(shell, "powershell") {
			cmd = []string{shell, "-Command", command}
		} else {
			cmd = []string{shell, "/c", command}
		}
	} else {
		cmd = []string{shell, "-c", command}
	}

	return e.Execute(ctx, cmd)
}

// findShell finds an available shell from the allowed list.
func (e *LocalExecutor) findShell() string {
	for _, shell := range e.AllowedShells {
		if path, err := exec.LookPath(shell); err == nil {
			return path
		}
	}
	return ""
}

// IsAvailable checks if local execution is possible.
// Returns true if at least one shell is available.
func (e *LocalExecutor) IsAvailable() bool {
	return e.findShell() != ""
}

// SetTimeout updates the timeout.
func (e *LocalExecutor) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		e.Timeout = timeout
	}
}

// SetWorkDir updates the working directory.
func (e *LocalExecutor) SetWorkDir(dir string) error {
	if dir == "" {
		e.WorkDir = ""
		return nil
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid directory: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("directory does not exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absDir)
	}

	e.WorkDir = absDir
	return nil
}

// AddEnv adds an environment variable.
func (e *LocalExecutor) AddEnv(key, value string) {
	e.Env = append(e.Env, fmt.Sprintf("%s=%s", key, value))
}

// ClearEnv clears all custom environment variables.
func (e *LocalExecutor) ClearEnv() {
	e.Env = nil
}

// limitedWriter is a writer that limits the amount of data written.
type limitedWriter struct {
	w       *bytes.Buffer
	limit   int
	written int
}

func (lw *limitedWriter) Write(p []byte) (n int, err error) {
	originalLen := len(p)

	if lw.written >= lw.limit {
		return originalLen, nil // Silently discard
	}

	remaining := lw.limit - lw.written
	if len(p) > remaining {
		p = p[:remaining]
	}

	n, err = lw.w.Write(p)
	lw.written += n
	return originalLen, err // Report full length written
}

// Executor is the interface for command execution.
// Both Sandbox and LocalExecutor implement this interface.
type Executor interface {
	Execute(ctx context.Context, cmd []string) (stdout, stderr string, exitCode int, err error)
	ExecuteShell(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error)
}

// Ensure types implement Executor interface.
var (
	_ Executor = (*Sandbox)(nil)
	_ Executor = (*LocalExecutor)(nil)
)

// NewExecutor creates the appropriate executor based on Docker availability.
// If Docker is available and working, returns a Sandbox executor.
// Otherwise, returns a LocalExecutor as fallback.
func NewExecutor(cfg SandboxConfig) (Executor, error) {
	// Try to create a sandbox
	sandbox, err := New(cfg)
	if err != nil {
		// Docker client creation failed, use fallback
		return NewLocalExecutorWithConfig(cfg.WorkDir, cfg.Timeout), nil
	}

	// Check if Docker daemon is accessible
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sandbox.Ping(ctx); err != nil {
		// Docker daemon not accessible, use fallback
		_ = sandbox.Close()
		return NewLocalExecutorWithConfig(cfg.WorkDir, cfg.Timeout), nil
	}

	// Docker is available, start the sandbox
	if err := sandbox.Start(ctx); err != nil {
		// Failed to start sandbox, use fallback
		_ = sandbox.Close()
		return NewLocalExecutorWithConfig(cfg.WorkDir, cfg.Timeout), nil
	}

	return sandbox, nil
}

// MustNewExecutor is like NewExecutor but panics on error.
func MustNewExecutor(cfg SandboxConfig) Executor {
	executor, err := NewExecutor(cfg)
	if err != nil {
		panic(err)
	}
	return executor
}

// IsDockerAvailable checks if Docker is available and accessible.
func IsDockerAvailable() bool {
	sandbox, err := New(DefaultConfig())
	if err != nil {
		return false
	}
	defer sandbox.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return sandbox.Ping(ctx) == nil
}
