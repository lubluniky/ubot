package bus

import "time"

// InboundMessage represents a message received from any channel.
type InboundMessage struct {
	Channel   string                 `json:"channel"`   // telegram, whatsapp, cli, system
	SenderID  string                 `json:"senderId"`
	ChatID    string                 `json:"chatId"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Media     []string               `json:"media,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SessionKey returns a unique identifier for the conversation session.
func (m *InboundMessage) SessionKey() string {
	return m.Channel + ":" + m.ChatID
}

// OutboundMessage represents a message to be sent to a channel.
type OutboundMessage struct {
	Channel  string                 `json:"channel"`
	ChatID   string                 `json:"chatId"`
	Content  string                 `json:"content"`
	ReplyTo  string                 `json:"replyTo,omitempty"`
	Media    []string               `json:"media,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
