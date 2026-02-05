package session

import (
	"sync"
	"time"
)

// Message represents a single message in a conversation
type Message struct {
	Role       string         `json:"role"`                 // user, assistant, system, tool
	Content    string         `json:"content"`
	Timestamp  time.Time      `json:"timestamp"`
	ToolCalls  []ToolCallInfo `json:"toolCalls,omitempty"`
	ToolCallID string         `json:"toolCallId,omitempty"`
	Name       string         `json:"name,omitempty"` // for tool results
}

// ToolCallInfo contains information about a tool call made by the assistant
type ToolCallInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Session represents a conversation session with history
type Session struct {
	Key       string                 `json:"key"`                // channel:chatId
	Source    string                 `json:"source,omitempty"`   // "cli", "telegram", "whatsapp"
	Messages  []Message              `json:"messages"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	mu        sync.RWMutex
}

// NewSession creates a new session with the given key
func NewSession(key string) *Session {
	now := time.Now()
	return &Session{
		Key:       key,
		Messages:  make([]Message, 0),
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]interface{}),
	}
}

// AddMessage adds a new message with the given role and content
func (s *Session) AddMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// AddToolCall adds an assistant message with tool calls
func (s *Session) AddToolCall(toolCalls []ToolCallInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := Message{
		Role:      "assistant",
		Content:   "",
		Timestamp: time.Now(),
		ToolCalls: toolCalls,
	}
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// AddToolResult adds a tool result message
func (s *Session) AddToolResult(toolCallID, name, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := Message{
		Role:       "tool",
		Content:    result,
		Timestamp:  time.Now(),
		ToolCallID: toolCallID,
		Name:       name,
	}
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// GetHistory returns the last maxMessages messages from the session
// If maxMessages <= 0, returns all messages
func (s *Session) GetHistory(maxMessages int) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxMessages <= 0 || maxMessages >= len(s.Messages) {
		// Return a copy to prevent external modification
		result := make([]Message, len(s.Messages))
		copy(result, s.Messages)
		return result
	}

	// Return the last maxMessages
	start := len(s.Messages) - maxMessages
	result := make([]Message, maxMessages)
	copy(result, s.Messages[start:])
	return result
}

// GetMessages returns a copy of all messages
func (s *Session) GetMessages() []Message {
	return s.GetHistory(0)
}

// Clear removes all messages from the session
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = make([]Message, 0)
	s.UpdatedAt = time.Now()
}

// MessageCount returns the number of messages in the session
func (s *Session) MessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.Messages)
}

// SessionInfo provides summary information about a session
type SessionInfo struct {
	Key          string                 `json:"key"`
	MessageCount int                    `json:"messageCount"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Info returns summary information about the session
func (s *Session) Info() SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return SessionInfo{
		Key:          s.Key,
		MessageCount: len(s.Messages),
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		Metadata:     s.Metadata,
	}
}
