package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxHistory = 50
	sessionFileExt    = ".jsonl"
)

// sessionMetadata is the first line of a session file
type sessionMetadata struct {
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Manager handles session storage and retrieval
type Manager struct {
	sessionsDir string
	cache       map[string]*Session
	mu          sync.RWMutex
	maxHistory  int
}

// NewManager creates a new session manager with the given data directory
func NewManager(dataDir string) *Manager {
	sessionsDir := filepath.Join(dataDir, "sessions")

	// Ensure sessions directory exists
	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		// Log error but continue - operations will fail gracefully
		fmt.Fprintf(os.Stderr, "warning: failed to create sessions directory: %v\n", err)
	}

	return &Manager{
		sessionsDir: sessionsDir,
		cache:       make(map[string]*Session),
		maxHistory:  defaultMaxHistory,
	}
}

// SetMaxHistory sets the maximum number of messages to keep in history
func (m *Manager) SetMaxHistory(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxHistory = max
}

// GetOrCreate returns an existing session or creates a new one
func (m *Manager) GetOrCreate(key string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check cache first
	if session, ok := m.cache[key]; ok {
		return session
	}

	// Try to load from file
	session := m.loadFromFile(key)
	if session == nil {
		// Create new session
		session = NewSession(key)
	}

	m.cache[key] = session
	return session
}

// Get returns a session if it exists, nil otherwise
func (m *Manager) Get(key string) *Session {
	m.mu.RLock()

	// Check cache first
	if session, ok := m.cache[key]; ok {
		m.mu.RUnlock()
		return session
	}
	m.mu.RUnlock()

	// Try to load from file
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check cache after acquiring write lock
	if session, ok := m.cache[key]; ok {
		return session
	}

	session := m.loadFromFile(key)
	if session != nil {
		m.cache[key] = session
	}
	return session
}

// Save persists a session to disk
func (m *Manager) Save(session *Session) error {
	if session == nil {
		return fmt.Errorf("cannot save nil session")
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	filePath := m.getFilePath(session.Key)

	// Create file
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer file.Close()

	// Write metadata as first line
	meta := sessionMetadata{
		Key:       session.Key,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if _, err := file.Write(append(metaJSON, '\n')); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Write messages
	// Only keep last maxHistory messages
	messages := session.Messages
	if m.maxHistory > 0 && len(messages) > m.maxHistory {
		messages = messages[len(messages)-m.maxHistory:]
	}

	for _, msg := range messages {
		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		if _, err := file.Write(append(msgJSON, '\n')); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}

	return nil
}

// Delete removes a session from cache and disk
func (m *Manager) Delete(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from cache
	delete(m.cache, key)

	// Remove file
	filePath := m.getFilePath(key)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return false
	}

	return true
}

// List returns information about all sessions
func (m *Manager) List() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []SessionInfo

	// Read session files from directory
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		return sessions
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), sessionFileExt) {
			continue
		}

		// Extract key from filename
		key := m.keyFromFilename(entry.Name())

		// Check cache first
		if session, ok := m.cache[key]; ok {
			sessions = append(sessions, session.Info())
			continue
		}

		// Load metadata from file
		filePath := filepath.Join(m.sessionsDir, entry.Name())
		info := m.loadSessionInfo(filePath)
		if info != nil {
			sessions = append(sessions, *info)
		}
	}

	return sessions
}

// Clear clears the history for a specific session
func (m *Manager) Clear(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear in cache if present
	if session, ok := m.cache[key]; ok {
		session.Clear()
		// Save the cleared session
		m.mu.Unlock()
		err := m.Save(session)
		m.mu.Lock()
		return err
	}

	// Remove file if it exists
	filePath := m.getFilePath(key)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear session: %w", err)
	}

	return nil
}

// ClearAll clears all sessions
func (m *Manager) ClearAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear cache
	m.cache = make(map[string]*Session)

	// Remove all session files
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var lastErr error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), sessionFileExt) {
			continue
		}
		filePath := filepath.Join(m.sessionsDir, entry.Name())
		if err := os.Remove(filePath); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// getFilePath returns the file path for a session key
func (m *Manager) getFilePath(key string) string {
	safeKey := m.safeKey(key)
	return filepath.Join(m.sessionsDir, safeKey+sessionFileExt)
}

// safeKey converts a session key to a safe filename
func (m *Manager) safeKey(key string) string {
	// Remove null bytes
	key = strings.ReplaceAll(key, "\x00", "")
	// Remove path traversal components
	key = strings.ReplaceAll(key, "..", "")
	// Remove path separators
	key = strings.ReplaceAll(key, "/", "")
	key = strings.ReplaceAll(key, "\\", "")
	// Replace ":" with "_" for filesystem safety
	return strings.ReplaceAll(key, ":", "_")
}

// keyFromFilename converts a filename back to a session key
func (m *Manager) keyFromFilename(filename string) string {
	// Remove extension
	name := strings.TrimSuffix(filename, sessionFileExt)
	// Replace "_" back to ":" (first occurrence only for channel prefix)
	// This is a simple heuristic - we assume format is "channel_chatId"
	return strings.Replace(name, "_", ":", 1)
}

// loadFromFile loads a session from disk
func (m *Manager) loadFromFile(key string) *Session {
	filePath := m.getFilePath(key)

	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Read metadata from first line
	if !scanner.Scan() {
		return nil
	}

	var meta sessionMetadata
	if err := json.Unmarshal(scanner.Bytes(), &meta); err != nil {
		return nil
	}

	session := &Session{
		Key:       meta.Key,
		Messages:  make([]Message, 0),
		CreatedAt: meta.CreatedAt,
		UpdatedAt: meta.UpdatedAt,
		Metadata:  make(map[string]interface{}),
	}

	// Read messages
	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue // Skip malformed messages
		}
		session.Messages = append(session.Messages, msg)
	}

	return session
}

// loadSessionInfo loads only the metadata from a session file
func (m *Manager) loadSessionInfo(filePath string) *SessionInfo {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Read metadata from first line
	if !scanner.Scan() {
		return nil
	}

	var meta sessionMetadata
	if err := json.Unmarshal(scanner.Bytes(), &meta); err != nil {
		return nil
	}

	// Count messages
	msgCount := 0
	for scanner.Scan() {
		msgCount++
	}

	return &SessionInfo{
		Key:          meta.Key,
		MessageCount: msgCount,
		CreatedAt:    meta.CreatedAt,
		UpdatedAt:    meta.UpdatedAt,
	}
}
