package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs.
// This works with OpenAI, OpenRouter, Anthropic via proxy, Groq, and other
// OpenAI-compatible endpoints.
type OpenAIProvider struct {
	name         string
	apiKey       string
	apiBase      string
	defaultModel string
	client       *http.Client
}

// openAIRequest represents the request body for OpenAI chat completions.
type openAIRequest struct {
	Model       string                   `json:"model"`
	Messages    []openAIMessage          `json:"messages"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
}

// openAIMessage represents a message in the OpenAI format.
type openAIMessage struct {
	Role       string            `json:"role"`
	Content    interface{}       `json:"content"`
	ToolCalls  []openAIToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	Name       string            `json:"name,omitempty"`
}

// openAIToolCall represents a tool call in the OpenAI format.
type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

// openAIFunctionCall represents the function details in a tool call.
type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openAIResponse represents the response from OpenAI chat completions.
type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// NewOpenAIProvider creates a new OpenAI-compatible provider.
func NewOpenAIProvider(name, apiKey, apiBase, defaultModel string) *OpenAIProvider {
	// Ensure apiBase doesn't have trailing slash
	apiBase = strings.TrimSuffix(apiBase, "/")

	return &OpenAIProvider{
		name:         name,
		apiKey:       apiKey,
		apiBase:      apiBase,
		defaultModel: defaultModel,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name returns the provider's name.
func (p *OpenAIProvider) Name() string {
	return p.name
}

// DefaultModel returns the provider's default model.
func (p *OpenAIProvider) DefaultModel() string {
	return p.defaultModel
}

// Chat sends a chat completion request to the OpenAI-compatible API.
func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert messages to OpenAI format
	messages := make([]openAIMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openAIMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			Name:       msg.Name,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			messages[i].ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				// Serialize arguments back to JSON string
				argsJSON, err := json.Marshal(tc.Arguments)
				if err != nil {
					argsJSON = []byte("{}")
				}
				messages[i].ToolCalls[j] = openAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openAIFunctionCall{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				}
			}
		}
	}

	// Build request body
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	openAIReq := openAIRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	// Convert tools if present
	if req.Tools != nil {
		if tools, ok := req.Tools.([]map[string]interface{}); ok {
			openAIReq.Tools = tools
		} else if tools, ok := req.Tools.([]interface{}); ok {
			openAIReq.Tools = make([]map[string]interface{}, len(tools))
			for i, t := range tools {
				if toolMap, ok := t.(map[string]interface{}); ok {
					openAIReq.Tools[i] = toolMap
				}
			}
		}
	}

	// Marshal request body
	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", p.apiBase)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	// Add provider-specific headers
	switch p.name {
	case "openrouter":
		httpReq.Header.Set("HTTP-Referer", "https://github.com/ubot")
		httpReq.Header.Set("X-Title", "uBot")
	case "anthropic":
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	}

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API-level errors
	if openAIResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", openAIResp.Error.Message)
	}

	// Check for empty choices
	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Extract the first choice
	choice := openAIResp.Choices[0]

	// Build response
	chatResp := &ChatResponse{
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
	}

	// Extract content (handle both string and structured content)
	if content, ok := choice.Message.Content.(string); ok {
		chatResp.Content = content
	} else if choice.Message.Content != nil {
		// Handle structured content by marshaling it back to string
		contentJSON, _ := json.Marshal(choice.Message.Content)
		chatResp.Content = string(contentJSON)
	}

	// Convert tool calls
	if len(choice.Message.ToolCalls) > 0 {
		chatResp.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			// Parse arguments from JSON string to map
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				// If parsing fails, store as raw string in a special key
				args = map[string]interface{}{"_raw": tc.Function.Arguments}
			}

			chatResp.ToolCalls[i] = ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			}
		}
	}

	return chatResp, nil
}
