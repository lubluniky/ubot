package channels

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/hkuds/ubot/internal/bus"
)

// Channel is the interface all channels must implement.
type Channel interface {
	// Name returns the unique identifier for this channel.
	Name() string

	// Start begins listening for messages on this channel.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the channel.
	Stop() error

	// Send delivers an outbound message through this channel.
	Send(msg bus.OutboundMessage) error

	// IsRunning returns true if the channel is currently active.
	IsRunning() bool
}

// BaseChannel provides common functionality for all channel implementations.
type BaseChannel struct {
	name      string
	bus       *bus.MessageBus
	allowList []string
	running   bool
	mu        sync.RWMutex
}

// NewBaseChannel creates a new BaseChannel with the given parameters.
func NewBaseChannel(name string, msgBus *bus.MessageBus, allowList []string) BaseChannel {
	return BaseChannel{
		name:      name,
		bus:       msgBus,
		allowList: allowList,
		running:   false,
	}
}

// Name returns the channel's unique identifier.
func (c *BaseChannel) Name() string {
	return c.name
}

// IsRunning returns true if the channel is currently active.
func (c *BaseChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// setRunning sets the running state of the channel.
func (c *BaseChannel) setRunning(running bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = running
}

// IsAllowed checks if a sender is permitted to use this channel.
// Returns true if:
// - The senderID matches any item in the allowList
// - For compound IDs like "123456|username", checks both the numeric ID and username
// Returns false if the allowList is empty (deny all by default).
func (c *BaseChannel) IsAllowed(senderID string) bool {
	// Empty allowList means deny everyone — no users configured
	if len(c.allowList) == 0 {
		log.Printf("[security] channel=%s action=denied reason=no_allowed_users sender=%s", c.name, senderID)
		return false
	}

	// Check if senderID directly matches any allowed ID
	for _, allowed := range c.allowList {
		if senderID == allowed {
			return true
		}
	}

	// Handle compound IDs like "123456|username"
	if strings.Contains(senderID, "|") {
		parts := strings.Split(senderID, "|")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			for _, allowed := range c.allowList {
				if part == allowed {
					return true
				}
			}
		}
	}

	return false
}

// publishInbound creates and publishes an inbound message to the message bus.
func (c *BaseChannel) publishInbound(senderID, chatID, content string, media []string, metadata map[string]interface{}) {
	msg := bus.InboundMessage{
		Channel:   c.name,
		SenderID:  senderID,
		ChatID:    chatID,
		Content:   content,
		Timestamp: time.Now(),
		Media:     media,
		Metadata:  metadata,
	}
	c.bus.PublishInbound(msg)
}

// getBus returns the message bus for use by derived channels.
func (c *BaseChannel) getBus() *bus.MessageBus {
	return c.bus
}
