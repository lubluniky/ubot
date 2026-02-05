package bus

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrTimeout is returned when a message receive operation times out.
var ErrTimeout = errors.New("timeout waiting for message")

// MessageBus provides a channel-based message passing system for inbound
// and outbound messages with subscriber support.
type MessageBus struct {
	inbound  chan InboundMessage
	outbound chan OutboundMessage

	subscribers map[string][]func(OutboundMessage)
	mu          sync.RWMutex

	closed chan struct{}
}

// NewMessageBus creates a new MessageBus with the specified buffer size
// for both inbound and outbound channels.
func NewMessageBus(bufferSize int) *MessageBus {
	return &MessageBus{
		inbound:     make(chan InboundMessage, bufferSize),
		outbound:    make(chan OutboundMessage, bufferSize),
		subscribers: make(map[string][]func(OutboundMessage)),
		closed:      make(chan struct{}),
	}
}

// PublishInbound sends a message to the inbound channel.
func (b *MessageBus) PublishInbound(msg InboundMessage) {
	select {
	case <-b.closed:
		return
	case b.inbound <- msg:
	}
}

// ConsumeInbound blocks until an inbound message is available.
func (b *MessageBus) ConsumeInbound() InboundMessage {
	return <-b.inbound
}

// ConsumeInboundWithTimeout waits for an inbound message with a timeout.
// Returns ErrTimeout if no message is received within the specified duration.
func (b *MessageBus) ConsumeInboundWithTimeout(ctx context.Context, timeout time.Duration) (InboundMessage, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case msg := <-b.inbound:
		return msg, nil
	case <-timer.C:
		return InboundMessage{}, ErrTimeout
	case <-ctx.Done():
		return InboundMessage{}, ctx.Err()
	}
}

// PublishOutbound sends a message to the outbound channel.
func (b *MessageBus) PublishOutbound(msg OutboundMessage) {
	select {
	case <-b.closed:
		return
	case b.outbound <- msg:
	}
}

// ConsumeOutbound blocks until an outbound message is available.
func (b *MessageBus) ConsumeOutbound() OutboundMessage {
	return <-b.outbound
}

// SubscribeOutbound registers a callback function to receive outbound
// messages for the specified channel.
func (b *MessageBus) SubscribeOutbound(channel string, callback func(OutboundMessage)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[channel] = append(b.subscribers[channel], callback)
}

// DispatchOutbound runs a goroutine that dispatches outbound messages
// to registered subscribers. It should be called once and will run until
// the context is cancelled.
func (b *MessageBus) DispatchOutbound(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.closed:
			return
		case msg := <-b.outbound:
			b.mu.RLock()
			callbacks := b.subscribers[msg.Channel]
			b.mu.RUnlock()

			for _, cb := range callbacks {
				go func(callback func(OutboundMessage)) {
					defer func() {
						if r := recover(); r != nil {
							// Panic recovered in subscriber callback
							// In production, this could be logged
						}
					}()
					callback(msg)
				}(cb)
			}
		}
	}
}

// InboundSize returns the current number of messages in the inbound channel.
func (b *MessageBus) InboundSize() int {
	return len(b.inbound)
}

// OutboundSize returns the current number of messages in the outbound channel.
func (b *MessageBus) OutboundSize() int {
	return len(b.outbound)
}

// Close closes the message bus, stopping all dispatch operations.
func (b *MessageBus) Close() {
	close(b.closed)
}
