package session

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	s := NewSession("telegram:123")
	if s.Key != "telegram:123" {
		t.Errorf("Key = %q, want %q", s.Key, "telegram:123")
	}
	if s.MessageCount() != 0 {
		t.Errorf("MessageCount() = %d, want 0", s.MessageCount())
	}
	if s.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestAddMessage(t *testing.T) {
	s := NewSession("test")
	s.AddMessage("user", "hello")
	s.AddMessage("assistant", "hi there")

	if s.MessageCount() != 2 {
		t.Fatalf("MessageCount() = %d, want 2", s.MessageCount())
	}

	msgs := s.GetMessages()
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("first message = %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi there" {
		t.Errorf("second message = %+v", msgs[1])
	}
}

func TestAddToolCall(t *testing.T) {
	s := NewSession("test")
	s.AddToolCall([]ToolCallInfo{
		{ID: "call_1", Name: "exec", Arguments: `{"command":"ls"}`},
	})

	if s.MessageCount() != 1 {
		t.Fatalf("MessageCount() = %d, want 1", s.MessageCount())
	}

	msgs := s.GetMessages()
	if msgs[0].Role != "assistant" {
		t.Errorf("Role = %q, want %q", msgs[0].Role, "assistant")
	}
	if len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(msgs[0].ToolCalls))
	}
	if msgs[0].ToolCalls[0].Name != "exec" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", msgs[0].ToolCalls[0].Name, "exec")
	}
}

func TestAddToolResult(t *testing.T) {
	s := NewSession("test")
	s.AddToolResult("call_1", "exec", "file1\nfile2")

	msgs := s.GetMessages()
	if msgs[0].Role != "tool" {
		t.Errorf("Role = %q, want %q", msgs[0].Role, "tool")
	}
	if msgs[0].ToolCallID != "call_1" {
		t.Errorf("ToolCallID = %q, want %q", msgs[0].ToolCallID, "call_1")
	}
	if msgs[0].Name != "exec" {
		t.Errorf("Name = %q, want %q", msgs[0].Name, "exec")
	}
}

func TestGetHistory(t *testing.T) {
	s := NewSession("test")
	for i := 0; i < 5; i++ {
		s.AddMessage("user", "msg")
	}

	// Get all
	all := s.GetHistory(0)
	if len(all) != 5 {
		t.Errorf("GetHistory(0) len = %d, want 5", len(all))
	}

	// Get last 3
	last3 := s.GetHistory(3)
	if len(last3) != 3 {
		t.Errorf("GetHistory(3) len = %d, want 3", len(last3))
	}

	// Get more than available
	all2 := s.GetHistory(10)
	if len(all2) != 5 {
		t.Errorf("GetHistory(10) len = %d, want 5", len(all2))
	}
}

func TestClear(t *testing.T) {
	s := NewSession("test")
	s.AddMessage("user", "hello")
	s.Clear()

	if s.MessageCount() != 0 {
		t.Errorf("MessageCount() after Clear = %d, want 0", s.MessageCount())
	}
}

func TestInfo(t *testing.T) {
	s := NewSession("cli:local")
	s.AddMessage("user", "test")
	s.AddMessage("assistant", "response")

	info := s.Info()
	if info.Key != "cli:local" {
		t.Errorf("Info().Key = %q, want %q", info.Key, "cli:local")
	}
	if info.MessageCount != 2 {
		t.Errorf("Info().MessageCount = %d, want 2", info.MessageCount)
	}
}

func TestGetMessagesReturnsCopy(t *testing.T) {
	s := NewSession("test")
	s.AddMessage("user", "original")

	msgs := s.GetMessages()
	msgs[0].Content = "modified"

	// Original should be unchanged
	original := s.GetMessages()
	if original[0].Content != "original" {
		t.Error("GetMessages should return a copy, not a reference")
	}
}

// --- Trim tests ---

func TestEstimateTokensForContent(t *testing.T) {
	// 4 chars per token
	got := EstimateTokensForContent("1234567890123456")
	if got != 4 {
		t.Errorf("EstimateTokensForContent() = %d, want 4", got)
	}
}

func TestEstimateTokens(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "hello"},
	}
	tokens := EstimateTokens(msgs)
	if tokens <= 0 {
		t.Errorf("EstimateTokens() = %d, want > 0", tokens)
	}
}

func TestTrimHistory(t *testing.T) {
	msgs := make([]Message, 10)
	for i := range msgs {
		msgs[i] = Message{Role: "user", Content: "hello world this is a test message"}
	}

	// No trim when limit is 0
	result := TrimHistory(msgs, 0)
	if len(result) != 10 {
		t.Errorf("TrimHistory(0) len = %d, want 10", len(result))
	}

	// Trim to fit small token budget
	result = TrimHistory(msgs, 50)
	if len(result) >= 10 {
		t.Errorf("TrimHistory(50) should have trimmed, got len %d", len(result))
	}
	if len(result) < 1 {
		t.Error("TrimHistory should keep at least 1 message")
	}

	// No trim when budget is large enough
	result = TrimHistory(msgs, 100000)
	if len(result) != 10 {
		t.Errorf("TrimHistory(100000) len = %d, want 10", len(result))
	}

	// Empty messages
	result = TrimHistory([]Message{}, 100)
	if len(result) != 0 {
		t.Errorf("TrimHistory(empty) len = %d, want 0", len(result))
	}
}

func TestTrimToMessageCount(t *testing.T) {
	msgs := make([]Message, 5)
	for i := range msgs {
		msgs[i] = Message{Role: "user", Content: "msg"}
	}

	result := TrimToMessageCount(msgs, 3)
	if len(result) != 3 {
		t.Errorf("TrimToMessageCount(3) len = %d, want 3", len(result))
	}

	// maxCount <= 0 returns all
	result = TrimToMessageCount(msgs, 0)
	if len(result) != 5 {
		t.Errorf("TrimToMessageCount(0) len = %d, want 5", len(result))
	}

	// maxCount >= len returns all
	result = TrimToMessageCount(msgs, 10)
	if len(result) != 5 {
		t.Errorf("TrimToMessageCount(10) len = %d, want 5", len(result))
	}
}

func TestTrimPreservingSystemMessages(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "hello world this is a test message with enough content"},
		{Role: "assistant", Content: "hi there this is a response message with enough content"},
		{Role: "user", Content: "another message here with some more text to make it longer"},
	}

	// With small budget, system messages should be preserved
	result := TrimPreservingSystemMessages(msgs, 50)
	hasSystem := false
	for _, m := range result {
		if m.Role == "system" {
			hasSystem = true
			break
		}
	}
	if !hasSystem {
		t.Error("TrimPreservingSystemMessages should preserve system messages")
	}

	// maxTokens <= 0 returns all
	result = TrimPreservingSystemMessages(msgs, 0)
	if len(result) != 4 {
		t.Errorf("TrimPreservingSystemMessages(0) len = %d, want 4", len(result))
	}

	// Empty messages
	result = TrimPreservingSystemMessages([]Message{}, 100)
	if len(result) != 0 {
		t.Errorf("TrimPreservingSystemMessages(empty) len = %d, want 0", len(result))
	}
}
