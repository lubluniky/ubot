package tools

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hkuds/ubot/internal/sandbox"
)

// ErrBlockedPath is returned when a tool tries to access a sensitive path.
type ErrBlockedPath struct {
	Path   string
	Reason string
}

func (e ErrBlockedPath) Error() string {
	return fmt.Sprintf("access denied: %s (%s)", e.Path, e.Reason)
}

// sensitiveDirectories are directory prefixes that should never be accessed.
var sensitiveDirectories = []string{
	".ssh",
	".gnupg",
	".aws",
	".azure",
	".gcloud",
	".config/gh",
	".kube",
	".docker",
}

// sensitiveFiles are specific filenames that should never be accessed.
var sensitiveFiles = []string{
	".netrc",
	".docker/config.json",
	".kube/config",
	".ubot/config.json",
}

// sensitiveAbsolutePaths are absolute paths that should never be accessed.
var sensitiveAbsolutePaths = []string{
	"/etc/shadow",
	"/etc/sudoers",
	"/proc/self/environ",
	"/proc/self/cmdline",
	"/proc/self/maps",
}

// sensitiveExtensions are file extensions that indicate sensitive content.
var sensitiveExtensions = []string{
	".pem",
	".key",
}

// sensitiveBasenames are filenames (without path) that indicate sensitive content.
var sensitiveBasenames = []string{
	".env",
	"credentials",
	"secrets",
}

// filesystemTools are tool names that operate on file paths.
var filesystemTools = map[string]bool{
	"read_file":  true,
	"write_file": true,
	"edit_file":  true,
	"list_dir":   true,
}

// SecureRegistry wraps a ToolRegistry and intercepts Execute calls
// to run security checks before delegating to the inner registry.
type SecureRegistry struct {
	inner        *ToolRegistry
	blockedPaths []string
}

// NewSecureRegistry creates a new SecureRegistry wrapping the given ToolRegistry.
func NewSecureRegistry(inner *ToolRegistry) *SecureRegistry {
	return &SecureRegistry{
		inner:        inner,
		blockedPaths: buildBlockedPaths(),
	}
}

// buildBlockedPaths constructs the list of blocked path prefixes by expanding
// home-relative paths to absolute paths. It also resolves symlinks so that
// comparisons work on systems where paths like /etc are symlinked (e.g., macOS
// where /etc -> /private/etc).
func buildBlockedPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	var paths []string

	if home != "" {
		for _, dir := range sensitiveDirectories {
			paths = append(paths, filepath.Join(home, dir))
		}
		for _, file := range sensitiveFiles {
			paths = append(paths, filepath.Join(home, file))
		}
	}

	for _, p := range sensitiveAbsolutePaths {
		paths = append(paths, p)
		// Also add symlink-resolved variant so checks work on systems like macOS
		// where /etc is a symlink to /private/etc.
		if resolved, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
			resolvedPath := filepath.Join(resolved, filepath.Base(p))
			if resolvedPath != p {
				paths = append(paths, resolvedPath)
			}
		}
	}

	return paths
}

// Execute runs security checks and then delegates to the inner registry.
func (s *SecureRegistry) Execute(ctx context.Context, name string, params map[string]interface{}) (string, error) {
	start := time.Now()

	// Look up the tool to validate params against its schema
	tool := s.inner.Get(name)
	if tool == nil {
		return "", ErrToolNotFound{Name: name}
	}

	// Validate parameters against the tool's JSON schema
	if errs := ValidateParams(params, tool.Parameters()); len(errs) > 0 {
		log.Printf("[security] tool=%s action=param_validation_failed errors=%v", name, errs)
		return "", fmt.Errorf("parameter validation failed: %s", strings.Join(errs, "; "))
	}

	// Path validation for filesystem tools
	if filesystemTools[name] {
		if err := s.validatePath(params); err != nil {
			log.Printf("[security] tool=%s action=blocked_path path=%s", name, redactParams(params))
			return "", err
		}
	}

	// Command validation for exec tool using sandbox.GuardCommand
	if name == "exec" {
		if err := s.validateExecCommand(params); err != nil {
			log.Printf("[security] tool=%s action=blocked_command params=%s", name, redactParams(params))
			return "", err
		}
	}

	// Delegate to the inner registry
	result, err := s.inner.Execute(ctx, name, params)

	// Audit log
	status := "ok"
	if err != nil {
		status = "error"
	}
	log.Printf("[security] tool=%s status=%s duration=%s params=%s",
		name, status, time.Since(start).Round(time.Millisecond), redactParams(params))

	return result, err
}

// validatePath checks that the file path in params does not point to a sensitive location.
func (s *SecureRegistry) validatePath(params map[string]interface{}) error {
	pathStr, err := GetStringParam(params, "path")
	if err != nil {
		return nil // No path param; let the tool itself handle the error
	}

	resolved, err := resolvePath(pathStr)
	if err != nil {
		return fmt.Errorf("cannot resolve path %q: %w", pathStr, err)
	}

	// Check against blocked path prefixes (sensitive directories and files)
	for _, blocked := range s.blockedPaths {
		if resolved == blocked || strings.HasPrefix(resolved, blocked+string(filepath.Separator)) {
			return ErrBlockedPath{Path: pathStr, Reason: "sensitive path"}
		}
	}

	// Check sensitive extensions
	ext := strings.ToLower(filepath.Ext(resolved))
	for _, sensitiveExt := range sensitiveExtensions {
		if ext == sensitiveExt {
			return ErrBlockedPath{Path: pathStr, Reason: "sensitive file type " + sensitiveExt}
		}
	}

	// Check sensitive basenames
	base := filepath.Base(resolved)
	for _, sensitiveBase := range sensitiveBasenames {
		if strings.EqualFold(base, sensitiveBase) {
			return ErrBlockedPath{Path: pathStr, Reason: "sensitive filename " + sensitiveBase}
		}
	}

	return nil
}

// validateExecCommand uses sandbox.GuardCommand to check if the command is safe.
func (s *SecureRegistry) validateExecCommand(params map[string]interface{}) error {
	command, err := GetStringParam(params, "command")
	if err != nil {
		return nil // No command param; let the tool itself handle the error
	}

	if reason := sandbox.GuardCommand(command); reason != "" {
		return fmt.Errorf("exec blocked: %s", reason)
	}

	return nil
}

// resolvePath expands ~ and resolves symlinks to get the real absolute path.
func resolvePath(path string) (string, error) {
	// Expand ~
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}

	// Clean the path first
	path = filepath.Clean(path)

	// Make absolute
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = abs
	}

	// Try to resolve symlinks. If the file doesn't exist yet (e.g., write_file),
	// resolve the parent directory instead.
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// File may not exist yet; resolve parent
		parent := filepath.Dir(path)
		resolvedParent, parentErr := filepath.EvalSymlinks(parent)
		if parentErr != nil {
			// Parent doesn't exist either; use the cleaned absolute path
			return path, nil
		}
		return filepath.Join(resolvedParent, filepath.Base(path)), nil
	}

	return resolved, nil
}

// redactParams returns a string representation of params with sensitive values redacted.
func redactParams(params map[string]interface{}) string {
	redacted := make(map[string]string, len(params))
	for k, v := range params {
		switch k {
		case "content":
			if s, ok := v.(string); ok {
				if len(s) > 50 {
					redacted[k] = fmt.Sprintf("[%d chars]", len(s))
				} else {
					redacted[k] = s
				}
			} else {
				redacted[k] = "[redacted]"
			}
		default:
			redacted[k] = fmt.Sprintf("%v", v)
		}
	}
	return fmt.Sprintf("%v", redacted)
}

// GetDefinitions delegates to the inner registry.
func (s *SecureRegistry) GetDefinitions() []ToolDefinition {
	return s.inner.GetDefinitions()
}

// Get delegates to the inner registry.
func (s *SecureRegistry) Get(name string) Tool {
	return s.inner.Get(name)
}

// List delegates to the inner registry.
func (s *SecureRegistry) List() []string {
	return s.inner.List()
}

// Has delegates to the inner registry.
func (s *SecureRegistry) Has(name string) bool {
	return s.inner.Has(name)
}

// Count delegates to the inner registry.
func (s *SecureRegistry) Count() int {
	return s.inner.Count()
}

// Register delegates to the inner registry.
func (s *SecureRegistry) Register(t Tool) error {
	return s.inner.Register(t)
}

// Inner returns the underlying ToolRegistry.
func (s *SecureRegistry) Inner() *ToolRegistry {
	return s.inner
}
