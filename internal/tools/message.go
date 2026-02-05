// Package tools provides the interface and utilities for agent tools.
package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// OutboundMessage represents a message to be sent to a channel.
type OutboundMessage struct {
	Content string `json:"content"`
	Channel string `json:"channel"`
	ChatID  string `json:"chat_id"`
}

// MessageTool sends messages to channels.
type MessageTool struct {
	BaseTool
	SendCallback func(OutboundMessage) error
	Channel      string
	ChatID       string
	mu           sync.RWMutex
}

// NewMessageTool creates a new MessageTool with the given send callback.
func NewMessageTool(sendCallback func(OutboundMessage) error) *MessageTool {
	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The message content to send",
			},
			"channel": map[string]interface{}{
				"type":        "string",
				"description": "The channel to send the message to (optional, uses current context if not specified)",
			},
			"chat_id": map[string]interface{}{
				"type":        "string",
				"description": "The chat/conversation ID to send the message to (optional, uses current context if not specified)",
			},
		},
		"required": []string{"content"},
	}

	return &MessageTool{
		BaseTool: NewBaseTool(
			"message",
			"Send a message to a channel or chat. If channel and chat_id are not specified, sends to the current conversation context.",
			parameters,
		),
		SendCallback: sendCallback,
	}
}

// SetContext sets the current channel and chat ID context.
// This is typically called when processing an incoming message to set up
// the reply context for the message tool.
func (t *MessageTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Channel = channel
	t.ChatID = chatID
}

// GetContext returns the current channel and chat ID context.
func (t *MessageTool) GetContext() (channel, chatID string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Channel, t.ChatID
}

// ClearContext clears the current context.
func (t *MessageTool) ClearContext() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Channel = ""
	t.ChatID = ""
}

// Execute sends a message using the configured callback.
func (t *MessageTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	// Check if callback is configured
	if t.SendCallback == nil {
		return "", errors.New("message: send callback not configured")
	}

	// Extract content (required)
	content, err := GetStringParam(params, "content")
	if err != nil {
		return "", fmt.Errorf("message: %w", err)
	}

	if content == "" {
		return "", errors.New("message: content cannot be empty")
	}

	// Get current context
	t.mu.RLock()
	currentChannel := t.Channel
	currentChatID := t.ChatID
	t.mu.RUnlock()

	// Extract optional channel and chat_id, defaulting to context
	channel := GetStringParamOr(params, "channel", currentChannel)
	chatID := GetStringParamOr(params, "chat_id", currentChatID)

	// Validate we have at least a channel
	if channel == "" {
		return "", errors.New("message: no channel specified and no context available")
	}

	// Build outbound message
	msg := OutboundMessage{
		Content: content,
		Channel: channel,
		ChatID:  chatID,
	}

	// Check context cancellation before sending
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("message: cancelled: %w", ctx.Err())
	default:
	}

	// Send the message
	if err := t.SendCallback(msg); err != nil {
		return "", fmt.Errorf("message: failed to send: %w", err)
	}

	// Build success response
	if chatID != "" {
		return fmt.Sprintf("Message sent to channel %q (chat: %s)", channel, chatID), nil
	}
	return fmt.Sprintf("Message sent to channel %q", channel), nil
}

// SetSendCallback updates the send callback function.
func (t *MessageTool) SetSendCallback(callback func(OutboundMessage) error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.SendCallback = callback
}

// WithContext returns a new MessageTool instance with the given context.
// This is useful for creating context-specific instances without modifying
// the original tool.
func (t *MessageTool) WithContext(channel, chatID string) *MessageTool {
	return &MessageTool{
		BaseTool:     t.BaseTool,
		SendCallback: t.SendCallback,
		Channel:      channel,
		ChatID:       chatID,
	}
}
