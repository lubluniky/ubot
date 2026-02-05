// Package tools provides filesystem operation tools for the agent.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// expandPath expands ~ to the user's home directory.
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}
	return filepath.Clean(path), nil
}

// ReadFileTool reads the contents of a file.
type ReadFileTool struct {
	BaseTool
}

// NewReadFileTool creates a new ReadFileTool.
func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{
		BaseTool: NewBaseTool(
			"read_file",
			"Read the contents of a file at the specified path. Supports ~ expansion for home directory.",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read. Supports ~ for home directory.",
					},
				},
				"required": []string{"path"},
			},
		),
	}
}

// Execute reads the file contents.
func (t *ReadFileTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	path, err := GetStringParam(params, "path")
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}

	expandedPath, err := expandPath(path)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}

	// Check if file exists
	info, err := os.Stat(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("read_file: file not found: %s", expandedPath)
		}
		return "", fmt.Errorf("read_file: cannot access file %s: %w", expandedPath, err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("read_file: path is a directory, not a file: %s", expandedPath)
	}

	content, err := os.ReadFile(expandedPath)
	if err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("read_file: permission denied: %s", expandedPath)
		}
		return "", fmt.Errorf("read_file: failed to read file %s: %w", expandedPath, err)
	}

	return string(content), nil
}

// WriteFileTool writes content to a file.
type WriteFileTool struct {
	BaseTool
}

// NewWriteFileTool creates a new WriteFileTool.
func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{
		BaseTool: NewBaseTool(
			"write_file",
			"Write content to a file at the specified path. Creates parent directories if they don't exist. Supports ~ expansion for home directory.",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to write. Supports ~ for home directory.",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to write to the file.",
					},
				},
				"required": []string{"path", "content"},
			},
		),
	}
}

// Execute writes content to the file.
func (t *WriteFileTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	path, err := GetStringParam(params, "path")
	if err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}

	content, err := GetStringParam(params, "content")
	if err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}

	expandedPath, err := expandPath(path)
	if err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}

	// Create parent directories if needed
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("write_file: permission denied creating directories for %s", expandedPath)
		}
		return "", fmt.Errorf("write_file: failed to create directories for %s: %w", expandedPath, err)
	}

	if err := os.WriteFile(expandedPath, []byte(content), 0644); err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("write_file: permission denied writing to %s", expandedPath)
		}
		return "", fmt.Errorf("write_file: failed to write file %s: %w", expandedPath, err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), expandedPath), nil
}

// EditFileTool replaces text in a file.
type EditFileTool struct {
	BaseTool
}

// NewEditFileTool creates a new EditFileTool.
func NewEditFileTool() *EditFileTool {
	return &EditFileTool{
		BaseTool: NewBaseTool(
			"edit_file",
			"Edit a file by replacing exact text. Finds and replaces the specified old_text with new_text. Only replaces the first occurrence unless the text appears multiple times (will warn).",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to edit. Supports ~ for home directory.",
					},
					"old_text": map[string]interface{}{
						"type":        "string",
						"description": "The exact text to find and replace.",
					},
					"new_text": map[string]interface{}{
						"type":        "string",
						"description": "The text to replace old_text with.",
					},
				},
				"required": []string{"path", "old_text", "new_text"},
			},
		),
	}
}

// Execute performs the text replacement in the file.
func (t *EditFileTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	path, err := GetStringParam(params, "path")
	if err != nil {
		return "", fmt.Errorf("edit_file: %w", err)
	}

	oldText, err := GetStringParam(params, "old_text")
	if err != nil {
		return "", fmt.Errorf("edit_file: %w", err)
	}

	newText, err := GetStringParam(params, "new_text")
	if err != nil {
		return "", fmt.Errorf("edit_file: %w", err)
	}

	if oldText == "" {
		return "", fmt.Errorf("edit_file: old_text cannot be empty")
	}

	expandedPath, err := expandPath(path)
	if err != nil {
		return "", fmt.Errorf("edit_file: %w", err)
	}

	// Check if file exists
	info, err := os.Stat(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("edit_file: file not found: %s", expandedPath)
		}
		return "", fmt.Errorf("edit_file: cannot access file %s: %w", expandedPath, err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("edit_file: path is a directory, not a file: %s", expandedPath)
	}

	content, err := os.ReadFile(expandedPath)
	if err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("edit_file: permission denied reading %s", expandedPath)
		}
		return "", fmt.Errorf("edit_file: failed to read file %s: %w", expandedPath, err)
	}

	contentStr := string(content)

	// Count occurrences
	count := strings.Count(contentStr, oldText)
	if count == 0 {
		return "", fmt.Errorf("edit_file: old_text not found in file %s", expandedPath)
	}

	// Perform replacement (only first occurrence)
	newContent := strings.Replace(contentStr, oldText, newText, 1)

	if err := os.WriteFile(expandedPath, []byte(newContent), 0644); err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("edit_file: permission denied writing to %s", expandedPath)
		}
		return "", fmt.Errorf("edit_file: failed to write file %s: %w", expandedPath, err)
	}

	if count > 1 {
		return fmt.Sprintf("Warning: Found %d matches of old_text, replaced only the first occurrence in %s", count, expandedPath), nil
	}

	return fmt.Sprintf("Successfully replaced text in %s", expandedPath), nil
}

// ListDirTool lists the contents of a directory.
type ListDirTool struct {
	BaseTool
}

// NewListDirTool creates a new ListDirTool.
func NewListDirTool() *ListDirTool {
	return &ListDirTool{
		BaseTool: NewBaseTool(
			"list_dir",
			"List the contents of a directory. Shows directories with [DIR] prefix and files with their size. Supports ~ expansion for home directory.",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the directory to list. Supports ~ for home directory.",
					},
				},
				"required": []string{"path"},
			},
		),
	}
}

// Execute lists the directory contents.
func (t *ListDirTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	path, err := GetStringParam(params, "path")
	if err != nil {
		return "", fmt.Errorf("list_dir: %w", err)
	}

	expandedPath, err := expandPath(path)
	if err != nil {
		return "", fmt.Errorf("list_dir: %w", err)
	}

	// Check if path exists and is a directory
	info, err := os.Stat(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("list_dir: directory not found: %s", expandedPath)
		}
		return "", fmt.Errorf("list_dir: cannot access path %s: %w", expandedPath, err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("list_dir: path is not a directory: %s", expandedPath)
	}

	entries, err := os.ReadDir(expandedPath)
	if err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("list_dir: permission denied: %s", expandedPath)
		}
		return "", fmt.Errorf("list_dir: failed to read directory %s: %w", expandedPath, err)
	}

	if len(entries) == 0 {
		return fmt.Sprintf("Directory %s is empty", expandedPath), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Contents of %s:\n", expandedPath))

	for _, entry := range entries {
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("[DIR]  %s/\n", entry.Name()))
		} else {
			// Get file info for size
			info, err := entry.Info()
			if err != nil {
				result.WriteString(fmt.Sprintf("[FILE] %s\n", entry.Name()))
			} else {
				result.WriteString(fmt.Sprintf("[FILE] %s (%s)\n", entry.Name(), formatSize(info.Size())))
			}
		}
	}

	return result.String(), nil
}

// formatSize formats a file size in human-readable form.
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}
