package providers

import "context"

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // string or []ContentPart for images
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

// Usage represents token usage statistics from the LLM response.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse represents the response from an LLM chat completion.
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	Usage        Usage      `json:"usage"`
}

// HasToolCalls returns true if the response contains tool calls.
func (r *ChatResponse) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}

// ChatRequest represents a request to the LLM for chat completion.
type ChatRequest struct {
	Messages    []ChatMessage `json:"messages"`
	Tools       interface{}   `json:"tools,omitempty"` // []ToolDefinition
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

// Provider defines the interface for LLM providers.
type Provider interface {
	// Name returns the provider's name.
	Name() string

	// Chat sends a chat completion request and returns the response.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// DefaultModel returns the provider's default model identifier.
	DefaultModel() string
}
