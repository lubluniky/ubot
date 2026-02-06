// Package agent implements the core agent loop and processing engine.
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryStore provides persistent memory storage for the agent.
// It manages MEMORY.md for long-term memory and daily notes for
// ephemeral day-to-day information.
type MemoryStore struct {
	workspace string
}

// NewMemoryStore creates a new MemoryStore with the given workspace path.
func NewMemoryStore(workspace string) *MemoryStore {
	return &MemoryStore{
		workspace: workspace,
	}
}

// GetMemoryContext returns content from MEMORY.md for the system prompt.
// Returns empty string if the file doesn't exist or cannot be read.
func (m *MemoryStore) GetMemoryContext() string {
	memoryPath := filepath.Join(m.workspace, "MEMORY.md")
	content, err := os.ReadFile(memoryPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

// GetDailyNotes returns today's notes from memory/YYYY-MM-DD.md.
// Returns empty string if the file doesn't exist or cannot be read.
func (m *MemoryStore) GetDailyNotes() string {
	notesPath := m.dailyNotesPath()
	content, err := os.ReadFile(notesPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

// AppendToMemory appends content to MEMORY.md.
// Creates the file if it doesn't exist.
func (m *MemoryStore) AppendToMemory(content string) error {
	memoryPath := filepath.Join(m.workspace, "MEMORY.md")

	// Ensure workspace directory exists
	if err := os.MkdirAll(m.workspace, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Open file in append mode, create if not exists
	f, err := os.OpenFile(memoryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open MEMORY.md: %w", err)
	}
	defer f.Close()

	// Add newline before content if file is not empty
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat MEMORY.md: %w", err)
	}

	var prefix string
	if info.Size() > 0 {
		prefix = "\n\n"
	}

	// Write content with timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = f.WriteString(fmt.Sprintf("%s[%s]\n%s", prefix, timestamp, content))
	if err != nil {
		return fmt.Errorf("failed to write to MEMORY.md: %w", err)
	}

	return nil
}

// AppendToDailyNotes appends content to today's daily notes file.
// Creates the memory directory and file if they don't exist.
func (m *MemoryStore) AppendToDailyNotes(content string) error {
	notesPath := m.dailyNotesPath()
	notesDir := filepath.Dir(notesPath)

	// Ensure memory directory exists
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Open file in append mode, create if not exists
	f, err := os.OpenFile(notesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open daily notes: %w", err)
	}
	defer f.Close()

	// Add newline before content if file is not empty
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat daily notes: %w", err)
	}

	var prefix string
	if info.Size() > 0 {
		prefix = "\n\n"
	}

	// Write content with timestamp
	timestamp := time.Now().Format("15:04:05")
	_, err = f.WriteString(fmt.Sprintf("%s[%s] %s", prefix, timestamp, content))
	if err != nil {
		return fmt.Errorf("failed to write to daily notes: %w", err)
	}

	return nil
}

// dailyNotesPath returns the path to today's daily notes file.
func (m *MemoryStore) dailyNotesPath() string {
	today := time.Now().Format("2006-01-02")
	return filepath.Join(m.workspace, "memory", today+".md")
}

// ClearDailyNotes removes today's daily notes file.
func (m *MemoryStore) ClearDailyNotes() error {
	notesPath := m.dailyNotesPath()
	err := os.Remove(notesPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear daily notes: %w", err)
	}
	return nil
}

// ListDailyNotes returns a list of all daily notes files (dates).
func (m *MemoryStore) ListDailyNotes() ([]string, error) {
	memoryDir := filepath.Join(m.workspace, "memory")

	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list memory directory: %w", err)
	}

	var dates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") {
			// Remove .md suffix to get the date
			date := strings.TrimSuffix(name, ".md")
			dates = append(dates, date)
		}
	}

	return dates, nil
}

// GetDailyNotesForDate returns notes for a specific date.
func (m *MemoryStore) GetDailyNotesForDate(date string) string {
	notesPath := filepath.Join(m.workspace, "memory", date+".md")
	content, err := os.ReadFile(notesPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}
