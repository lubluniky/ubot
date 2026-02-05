// Package tools provides the interface and utilities for agent tools.
package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Default configuration values for ExecTool.
const (
	DefaultExecTimeout = 60 * time.Second
	MaxOutputLength    = 10000
)

// ErrBlockedCommand is returned when a command matches a blocked pattern.
type ErrBlockedCommand struct {
	Command string
	Pattern string
}

func (e ErrBlockedCommand) Error() string {
	return fmt.Sprintf("command blocked: %q matches dangerous pattern %q", e.Command, e.Pattern)
}

// ErrInvalidWorkingDir is returned when the working directory is invalid.
type ErrInvalidWorkingDir struct {
	Dir string
	Err error
}

func (e ErrInvalidWorkingDir) Error() string {
	return fmt.Sprintf("invalid working directory %q: %v", e.Dir, e.Err)
}

func (e ErrInvalidWorkingDir) Unwrap() error {
	return e.Err
}

// ErrCommandTimeout is returned when a command exceeds its timeout.
type ErrCommandTimeout struct {
	Command string
	Timeout time.Duration
}

func (e ErrCommandTimeout) Error() string {
	return fmt.Sprintf("command %q timed out after %v", e.Command, e.Timeout)
}

// blockedPatterns contains regex patterns for dangerous commands.
var blockedPatterns = []*regexp.Regexp{
	// Destructive file operations
	regexp.MustCompile(`(?i)\brm\s+(-[a-z]*)?-[a-z]*r[a-z]*\s+(-[a-z]*\s+)*(/|~|\$HOME)\s*$`), // rm -rf /
	regexp.MustCompile(`(?i)\brm\s+(-[a-z]*\s+)*--no-preserve-root`),                          // rm --no-preserve-root
	regexp.MustCompile(`(?i)\brm\s+(-[a-z]*)?-[a-z]*r[a-z]*\s+(-[a-z]*\s+)*/\*`),              // rm -rf /*

	// Disk/filesystem destruction
	regexp.MustCompile(`(?i)\bdd\s+.*\bof\s*=\s*/dev/(sd[a-z]|hd[a-z]|nvme|vd[a-z])`), // dd to disk devices
	regexp.MustCompile(`(?i)\bmkfs\b`),                                                 // mkfs (format filesystem)
	regexp.MustCompile(`(?i)\bfdisk\b`),                                                // fdisk (partition management)
	regexp.MustCompile(`(?i)\bparted\b`),                                               // parted (partition management)
	regexp.MustCompile(`(?i)\bshred\b`),                                                // shred (secure delete)
	regexp.MustCompile(`(?i)\bwipefs\b`),                                               // wipefs (wipe filesystem signatures)
	regexp.MustCompile(`(?i)\bblkdiscard\b`),                                           // blkdiscard (discard device sectors)

	// Fork bombs and resource exhaustion
	regexp.MustCompile(`:\s*\(\s*\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;`),                     // classic fork bomb :(){ :|:& };:
	regexp.MustCompile(`(?i)\bwhile\s*\(\s*true\s*\)\s*;\s*do\s+fork`),                   // while true fork
	regexp.MustCompile(`(?i)\bfor\s*\(\s*;\s*;\s*\)\s*fork`),                             // infinite fork loop
	regexp.MustCompile(`(?i)/dev/zero\s*>\s*/dev/(?:sd[a-z]|hd[a-z]|nvme|vd[a-z]|mem)`), // write zeros to device

	// System damage
	regexp.MustCompile(`(?i)>\s*/dev/sd[a-z]`),               // redirect to disk
	regexp.MustCompile(`(?i)>\s*/dev/hd[a-z]`),               // redirect to disk
	regexp.MustCompile(`(?i)>\s*/dev/nvme`),                  // redirect to nvme
	regexp.MustCompile(`(?i)\bchmod\s+(-[a-z]*\s+)*777\s+/`), // chmod 777 /
	regexp.MustCompile(`(?i)\bchown\s+.*\s+/\s*$`),           // chown on root

	// Network attacks
	regexp.MustCompile(`(?i)\bcurl\s+.*\|\s*(ba)?sh`), // curl | sh (remote code execution)
	regexp.MustCompile(`(?i)\bwget\s+.*\|\s*(ba)?sh`), // wget | sh (remote code execution)
	regexp.MustCompile(`(?i)\bcurl\s+.*-o\s*/`),       // curl to root filesystem
	regexp.MustCompile(`(?i)\bwget\s+.*-O\s*/`),       // wget to root filesystem

	// Kernel/boot damage
	regexp.MustCompile(`(?i)\brm\s+.*(/boot/|/etc/passwd|/etc/shadow)`), // remove critical files
	regexp.MustCompile(`(?i)>\s*/proc/`),                                // write to proc
	regexp.MustCompile(`(?i)>\s*/sys/`),                                 // write to sys

	// Dangerous commands that should never run
	regexp.MustCompile(`(?i)\bhalt\b`),     // halt system
	regexp.MustCompile(`(?i)\bpoweroff\b`), // power off
	regexp.MustCompile(`(?i)\bshutdown\b`), // shutdown
	regexp.MustCompile(`(?i)\breboot\b`),   // reboot
	regexp.MustCompile(`(?i)\binit\s+0\b`), // init 0 (halt)
	regexp.MustCompile(`(?i)\binit\s+6\b`), // init 6 (reboot)
}

// ExecTool executes shell commands safely with timeout and safety checks.
type ExecTool struct {
	BaseTool
	Timeout             time.Duration
	WorkingDir          string
	RestrictToWorkspace bool
	allowedShells       []string
	blockedPatterns     []*regexp.Regexp
	maxOutputLength     int
}

// NewExecTool creates a new ExecTool with default configuration.
func NewExecTool() *ExecTool {
	return NewExecToolWithOptions(DefaultExecTimeout, "", false)
}

// NewExecToolWithOptions creates a new ExecTool with custom configuration.
func NewExecToolWithOptions(timeout time.Duration, workingDir string, restrictToWorkspace bool) *ExecTool {
	if timeout == 0 {
		timeout = DefaultExecTimeout
	}

	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"working_dir": map[string]interface{}{
				"type":        "string",
				"description": "Working directory for command execution (optional, defaults to current directory)",
			},
		},
		"required": []string{"command"},
	}

	return &ExecTool{
		BaseTool: NewBaseTool(
			"exec",
			"Execute shell commands safely with timeout support. Commands are validated against dangerous patterns before execution.",
			parameters,
		),
		Timeout:             timeout,
		WorkingDir:          workingDir,
		RestrictToWorkspace: restrictToWorkspace,
		allowedShells:       []string{"sh", "bash", "zsh"},
		maxOutputLength:     MaxOutputLength,
	}
}

// Execute runs the shell command with safety checks and timeout.
func (t *ExecTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	// Extract command parameter (required)
	command, err := GetStringParam(params, "command")
	if err != nil {
		return "", fmt.Errorf("exec: %w", err)
	}

	// Validate command is not empty
	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New("exec: command cannot be empty")
	}

	// Check for blocked patterns
	if err := t.validateCommand(command); err != nil {
		return "", err
	}

	// Extract working directory parameter (optional)
	workingDir := GetStringParamOr(params, "working_dir", t.WorkingDir)

	// Expand ~ in working directory
	if workingDir != "" {
		expandedDir, err := expandPath(workingDir)
		if err != nil {
			return "", fmt.Errorf("exec: %w", err)
		}
		workingDir = expandedDir
	}

	// Validate working directory if provided
	if workingDir != "" {
		if err := t.validateWorkingDir(workingDir); err != nil {
			return "", err
		}
	}

	// Check workspace restriction
	if t.RestrictToWorkspace && t.WorkingDir != "" && workingDir != "" {
		absWorkspace, _ := filepath.Abs(t.WorkingDir)
		absWorkDir, _ := filepath.Abs(workingDir)
		if !strings.HasPrefix(absWorkDir, absWorkspace) {
			return "", fmt.Errorf("exec: working directory %q is outside workspace %q", workingDir, t.WorkingDir)
		}
	}

	// Execute the command
	return t.executeCommand(ctx, command, workingDir)
}

// validateCommand checks if the command matches any blocked patterns.
func (t *ExecTool) validateCommand(command string) error {
	// Check against default blocked patterns
	for _, pattern := range blockedPatterns {
		if pattern.MatchString(command) {
			return ErrBlockedCommand{Command: command, Pattern: pattern.String()}
		}
	}

	// Check against custom blocked patterns
	for _, pattern := range t.blockedPatterns {
		if pattern.MatchString(command) {
			return ErrBlockedCommand{Command: command, Pattern: pattern.String()}
		}
	}

	return nil
}

// validateWorkingDir validates that the working directory exists and is accessible.
func (t *ExecTool) validateWorkingDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrInvalidWorkingDir{Dir: dir, Err: errors.New("directory does not exist")}
		}
		return ErrInvalidWorkingDir{Dir: dir, Err: err}
	}

	if !info.IsDir() {
		return ErrInvalidWorkingDir{Dir: dir, Err: errors.New("path is not a directory")}
	}

	return nil
}

// executeCommand runs the command with timeout and captures output.
func (t *ExecTool) executeCommand(ctx context.Context, command, workingDir string) (string, error) {
	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	// Find an available shell
	shell := t.findShell()
	if shell == "" {
		return "", errors.New("exec: no suitable shell found")
	}

	// Create the command
	cmd := exec.CommandContext(execCtx, shell, "-c", command)

	// Set working directory if provided
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()

	// Get exit code
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	// Build the output
	output := t.buildOutput(stdout.String(), stderr.String(), exitCode)

	// Check for context cancellation (timeout)
	if execCtx.Err() == context.DeadlineExceeded {
		return output, ErrCommandTimeout{Command: command, Timeout: t.Timeout}
	}

	// Check for context cancellation (parent context)
	if ctx.Err() != nil {
		return output, fmt.Errorf("exec: command cancelled: %w", ctx.Err())
	}

	// If the command failed, include the error but still return output
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Return output with exit code info, but don't treat non-zero exit as error
			// (the output already contains exit code info)
			return output, nil
		}
		return output, fmt.Errorf("exec: command execution failed: %w", err)
	}

	return output, nil
}

// findShell finds an available shell from the allowed list.
func (t *ExecTool) findShell() string {
	for _, shell := range t.allowedShells {
		if path, err := exec.LookPath(shell); err == nil {
			return path
		}
	}
	return ""
}

// buildOutput combines stdout, stderr, and exit code, truncating if necessary.
func (t *ExecTool) buildOutput(stdout, stderr string, exitCode int) string {
	var builder strings.Builder

	// Add stdout
	if stdout != "" {
		builder.WriteString(stdout)
	}

	// Add stderr with label if present
	if stderr != "" {
		if builder.Len() > 0 && !strings.HasSuffix(builder.String(), "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString("[stderr]\n")
		builder.WriteString(stderr)
	}

	// Add exit code if non-zero
	if exitCode != 0 {
		if builder.Len() > 0 && !strings.HasSuffix(builder.String(), "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("[exit code: %d]", exitCode))
	}

	output := builder.String()

	// Truncate if necessary
	if len(output) > t.maxOutputLength {
		truncated := output[:t.maxOutputLength]
		// Try to cut at a newline for cleaner output
		if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > t.maxOutputLength/2 {
			truncated = truncated[:lastNewline]
		}
		return truncated + fmt.Sprintf("\n... [output truncated, %d chars total]", len(output))
	}

	return output
}

// SetTimeout updates the timeout configuration.
func (t *ExecTool) SetTimeout(timeout time.Duration) {
	t.Timeout = timeout
}

// GetTimeout returns the current timeout configuration.
func (t *ExecTool) GetTimeout() time.Duration {
	return t.Timeout
}

// AddBlockedPattern adds a custom blocked pattern.
func (t *ExecTool) AddBlockedPattern(pattern *regexp.Regexp) {
	t.blockedPatterns = append(t.blockedPatterns, pattern)
}

// IsCommandBlocked checks if a command would be blocked without executing it.
func (t *ExecTool) IsCommandBlocked(command string) bool {
	return t.validateCommand(command) != nil
}

// SetWorkingDir sets the default working directory.
func (t *ExecTool) SetWorkingDir(dir string) {
	t.WorkingDir = dir
}

// SetRestrictToWorkspace enables or disables workspace restriction.
func (t *ExecTool) SetRestrictToWorkspace(restrict bool) {
	t.RestrictToWorkspace = restrict
}
