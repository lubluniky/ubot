package sandbox

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNewLocalExecutor(t *testing.T) {
	e := NewLocalExecutor()

	if e.WorkDir != DefaultFallbackWorkDir {
		t.Errorf("WorkDir = %q, want %q", e.WorkDir, DefaultFallbackWorkDir)
	}
	if e.Timeout != DefaultFallbackTimeout {
		t.Errorf("Timeout = %v, want %v", e.Timeout, DefaultFallbackTimeout)
	}
	if e.MaxOutputLen != MaxFallbackOutputLen {
		t.Errorf("MaxOutputLen = %d, want %d", e.MaxOutputLen, MaxFallbackOutputLen)
	}
	if len(e.AllowedShells) == 0 {
		t.Error("AllowedShells should not be empty")
	}
}

func TestNewLocalExecutorWithConfig(t *testing.T) {
	e := NewLocalExecutorWithConfig("/tmp", time.Minute)

	if e.WorkDir != "/tmp" {
		t.Errorf("WorkDir = %q, want %q", e.WorkDir, "/tmp")
	}
	if e.Timeout != time.Minute {
		t.Errorf("Timeout = %v, want %v", e.Timeout, time.Minute)
	}
}

func TestNewLocalExecutorWithConfigZeroTimeout(t *testing.T) {
	e := NewLocalExecutorWithConfig("", 0)

	if e.Timeout != DefaultFallbackTimeout {
		t.Errorf("Timeout = %v, want %v", e.Timeout, DefaultFallbackTimeout)
	}
}

func TestLocalExecutorExecute(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	e := NewLocalExecutor()
	ctx := context.Background()

	stdout, stderr, exitCode, err := e.Execute(ctx, []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hello\n")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestLocalExecutorExecuteShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	e := NewLocalExecutor()
	ctx := context.Background()

	stdout, stderr, exitCode, err := e.ExecuteShell(ctx, "echo hello && echo world")
	if err != nil {
		t.Fatalf("ExecuteShell failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if stdout != "hello\nworld\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hello\nworld\n")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestLocalExecutorExecuteWithStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	e := NewLocalExecutor()
	ctx := context.Background()

	stdout, stderr, exitCode, err := e.ExecuteShell(ctx, "echo stdout && echo stderr >&2")
	if err != nil {
		t.Fatalf("ExecuteShell failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if stdout != "stdout\n" {
		t.Errorf("stdout = %q, want %q", stdout, "stdout\n")
	}
	if stderr != "stderr\n" {
		t.Errorf("stderr = %q, want %q", stderr, "stderr\n")
	}
}

func TestLocalExecutorExecuteExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	e := NewLocalExecutor()
	ctx := context.Background()

	_, _, exitCode, err := e.ExecuteShell(ctx, "exit 42")
	if err != nil {
		t.Fatalf("ExecuteShell failed: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("exitCode = %d, want 42", exitCode)
	}
}

func TestLocalExecutorExecuteGuardBlocked(t *testing.T) {
	e := NewLocalExecutor()
	ctx := context.Background()

	_, _, _, err := e.Execute(ctx, []string{"rm", "-rf", "/"})
	if err == nil {
		t.Error("Execute should have failed for blocked command")
	}
}

func TestLocalExecutorExecuteShellGuardBlocked(t *testing.T) {
	e := NewLocalExecutor()
	ctx := context.Background()

	_, _, _, err := e.ExecuteShell(ctx, "rm -rf /")
	if err == nil {
		t.Error("ExecuteShell should have failed for blocked command")
	}
}

func TestLocalExecutorExecuteEmptyCommand(t *testing.T) {
	e := NewLocalExecutor()
	ctx := context.Background()

	_, _, _, err := e.Execute(ctx, []string{})
	if err == nil {
		t.Error("Execute should have failed for empty command")
	}
}

func TestLocalExecutorExecuteTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	e := NewLocalExecutor()
	e.Timeout = 100 * time.Millisecond
	ctx := context.Background()

	_, _, _, err := e.ExecuteShell(ctx, "sleep 10")
	if err == nil {
		t.Error("Execute should have timed out")
	}
}

func TestLocalExecutorExecuteWithWorkDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	tmpDir := t.TempDir()
	e := NewLocalExecutorWithConfig(tmpDir, DefaultFallbackTimeout)
	ctx := context.Background()

	stdout, _, _, err := e.ExecuteShell(ctx, "pwd")
	if err != nil {
		t.Fatalf("ExecuteShell failed: %v", err)
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	expectedDir, _ := filepath.EvalSymlinks(tmpDir)
	actualDir, _ := filepath.EvalSymlinks(stdout[:len(stdout)-1]) // Remove trailing newline

	if actualDir != expectedDir {
		t.Errorf("pwd = %q, want %q", actualDir, expectedDir)
	}
}

func TestLocalExecutorSetWorkDir(t *testing.T) {
	e := NewLocalExecutor()

	// Set to valid directory
	err := e.SetWorkDir(os.TempDir())
	if err != nil {
		t.Errorf("SetWorkDir failed: %v", err)
	}

	// Set to empty (reset)
	err = e.SetWorkDir("")
	if err != nil {
		t.Errorf("SetWorkDir('') failed: %v", err)
	}
	if e.WorkDir != "" {
		t.Errorf("WorkDir = %q, want empty", e.WorkDir)
	}

	// Set to non-existent directory
	err = e.SetWorkDir("/nonexistent/directory/path")
	if err == nil {
		t.Error("SetWorkDir should fail for non-existent directory")
	}
}

func TestLocalExecutorSetTimeout(t *testing.T) {
	e := NewLocalExecutor()

	e.SetTimeout(time.Minute)
	if e.Timeout != time.Minute {
		t.Errorf("Timeout = %v, want %v", e.Timeout, time.Minute)
	}

	// Zero timeout should not change
	originalTimeout := e.Timeout
	e.SetTimeout(0)
	if e.Timeout != originalTimeout {
		t.Errorf("Timeout = %v, want %v", e.Timeout, originalTimeout)
	}
}

func TestLocalExecutorAddEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	e := NewLocalExecutor()
	e.AddEnv("TEST_VAR", "test_value")
	ctx := context.Background()

	stdout, _, _, err := e.ExecuteShell(ctx, "echo $TEST_VAR")
	if err != nil {
		t.Fatalf("ExecuteShell failed: %v", err)
	}
	if stdout != "test_value\n" {
		t.Errorf("stdout = %q, want %q", stdout, "test_value\n")
	}
}

func TestLocalExecutorClearEnv(t *testing.T) {
	e := NewLocalExecutor()
	e.AddEnv("TEST_VAR", "test_value")
	e.ClearEnv()

	if len(e.Env) != 0 {
		t.Errorf("Env length = %d, want 0", len(e.Env))
	}
}

func TestLocalExecutorIsAvailable(t *testing.T) {
	e := NewLocalExecutor()

	if !e.IsAvailable() {
		t.Error("LocalExecutor should be available on most systems")
	}

	// Test with no shells
	e.AllowedShells = []string{}
	if e.IsAvailable() {
		t.Error("LocalExecutor should not be available with no shells")
	}
}

func TestLimitedWriter(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, limit: 10}

	// Write less than limit
	n, err := lw.Write([]byte("hello"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	// limitedWriter reports input length, not bytes actually written
	if n != 5 {
		t.Errorf("n = %d, want 5", n)
	}

	// Write more that causes truncation (only 5 more bytes will fit)
	input := []byte("world!!!!")
	n, err = lw.Write(input)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	// limitedWriter reports input length even when truncating
	if n != len(input) {
		t.Errorf("n = %d, want %d", n, len(input))
	}

	// Verify only first 10 bytes were written
	if buf.Len() != 10 {
		t.Errorf("buffer length = %d, want 10", buf.Len())
	}
	if buf.String() != "helloworld" {
		t.Errorf("buffer = %q, want %q", buf.String(), "helloworld")
	}

	// Write when already at limit - should be silently discarded
	n, err = lw.Write([]byte("more"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != 4 {
		t.Errorf("n = %d, want 4 (reported even if discarded)", n)
	}

	// Buffer should still be at 10
	if buf.Len() != 10 {
		t.Errorf("buffer length = %d, want 10 (no change)", buf.Len())
	}
}

func TestExecutorInterface(t *testing.T) {
	// Verify LocalExecutor implements Executor interface
	var _ Executor = (*LocalExecutor)(nil)
}

func TestDefaultShells(t *testing.T) {
	shells := defaultShells()
	if len(shells) == 0 {
		t.Error("defaultShells should return at least one shell")
	}

	if runtime.GOOS == "windows" {
		found := false
		for _, s := range shells {
			if s == "cmd" || s == "powershell" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Windows should have cmd or powershell in default shells")
		}
	} else {
		found := false
		for _, s := range shells {
			if s == "sh" || s == "bash" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Unix should have sh or bash in default shells")
		}
	}
}
