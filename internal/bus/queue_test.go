package bus

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSessionKey(t *testing.T) {
	msg := InboundMessage{Channel: "telegram", ChatID: "123"}
	if got := msg.SessionKey(); got != "telegram:123" {
		t.Errorf("SessionKey() = %q, want %q", got, "telegram:123")
	}
}

func TestNewMessageBus(t *testing.T) {
	bus := NewMessageBus(10)
	if bus == nil {
		t.Fatal("NewMessageBus returned nil")
	}
	if bus.InboundSize() != 0 {
		t.Errorf("InboundSize() = %d, want 0", bus.InboundSize())
	}
	if bus.OutboundSize() != 0 {
		t.Errorf("OutboundSize() = %d, want 0", bus.OutboundSize())
	}
}

func TestPublishConsumeInbound(t *testing.T) {
	bus := NewMessageBus(10)
	msg := InboundMessage{Channel: "cli", Content: "hello"}

	bus.PublishInbound(msg)

	if bus.InboundSize() != 1 {
		t.Errorf("InboundSize() = %d, want 1", bus.InboundSize())
	}

	got := bus.ConsumeInbound()
	if got.Content != "hello" {
		t.Errorf("ConsumeInbound().Content = %q, want %q", got.Content, "hello")
	}
}

func TestPublishConsumeOutbound(t *testing.T) {
	bus := NewMessageBus(10)
	msg := OutboundMessage{Channel: "telegram", ChatID: "42", Content: "response"}

	bus.PublishOutbound(msg)

	if bus.OutboundSize() != 1 {
		t.Errorf("OutboundSize() = %d, want 1", bus.OutboundSize())
	}

	got := bus.ConsumeOutbound()
	if got.Content != "response" {
		t.Errorf("ConsumeOutbound().Content = %q, want %q", got.Content, "response")
	}
}

func TestConsumeInboundWithTimeout(t *testing.T) {
	bus := NewMessageBus(10)

	// Timeout case
	ctx := context.Background()
	_, err := bus.ConsumeInboundWithTimeout(ctx, 10*time.Millisecond)
	if err != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %v", err)
	}

	// Success case
	bus.PublishInbound(InboundMessage{Content: "hi"})
	msg, err := bus.ConsumeInboundWithTimeout(ctx, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "hi" {
		t.Errorf("Content = %q, want %q", msg.Content, "hi")
	}

	// Context cancelled case
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = bus.ConsumeInboundWithTimeout(cancelCtx, time.Second)
	if err == nil {
		t.Error("expected context error, got nil")
	}
}

func TestSubscribeAndDispatchOutbound(t *testing.T) {
	bus := NewMessageBus(10)

	var received OutboundMessage
	var wg sync.WaitGroup
	wg.Add(1)

	bus.SubscribeOutbound("telegram", func(msg OutboundMessage) {
		received = msg
		wg.Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	go bus.DispatchOutbound(ctx)

	bus.PublishOutbound(OutboundMessage{Channel: "telegram", Content: "dispatched"})

	wg.Wait()
	cancel()

	if received.Content != "dispatched" {
		t.Errorf("received.Content = %q, want %q", received.Content, "dispatched")
	}
}

func TestCloseStopsPublish(t *testing.T) {
	// Fill the buffer so next publish would block
	bus := NewMessageBus(1)
	bus.PublishInbound(InboundMessage{Content: "fill"})
	bus.Close()

	// Should not block after close even when buffer is full
	done := make(chan struct{})
	go func() {
		bus.PublishInbound(InboundMessage{Content: "after close"})
		close(done)
	}()

	select {
	case <-done:
		// success - PublishInbound returned without blocking
	case <-time.After(time.Second):
		t.Fatal("PublishInbound blocked after Close")
	}
}
