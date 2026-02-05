package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// CopilotAPIEndpoint is the GitHub Copilot Chat API endpoint.
	CopilotAPIEndpoint = "https://api.githubcopilot.com/chat/completions"
	// CopilotIntegrationID identifies the integration to GitHub.
	CopilotIntegrationID = "vscode-chat"
	// CopilotDefaultModel is the default model for Copilot.
	CopilotDefaultModel = "gpt-4o"
)

// CopilotProvider implements the Provider interface for GitHub Copilot.
type CopilotProvider struct {
	accessToken string
	model       string
	client      *http.Client
}

// copilotRequest represents the request body for Copilot chat completions.
type copilotRequest struct {
	Model       string                   `json:"model"`
	Messages    []copilotMessage         `json:"messages"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
}

// copilotMessage represents a message in the Copilot format.
type copilotMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content"`
	ToolCalls  []copilotToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

// copilotToolCall represents a tool call in the Copilot format.
type copilotToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function copilotFunctionCall `json:"function"`
}

// copilotFunctionCall represents the function details in a tool call.
type copilotFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// copilotResponse represents the response from Copilot chat completions.
type copilotResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int            `json:"index"`
		Message      copilotMessage `json:"message"`
		FinishReason string         `json:"finish_reason"`
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

// NewCopilotProvider creates a new GitHub Copilot provider.
func NewCopilotProvider(accessToken, model string) *CopilotProvider {
	if model == "" {
		model = CopilotDefaultModel
	}

	return &CopilotProvider{
		accessToken: accessToken,
		model:       model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name returns the provider's name.
func (p *CopilotProvider) Name() string {
	return "copilot"
}

// DefaultModel returns the provider's default model.
func (p *CopilotProvider) DefaultModel() string {
	return p.model
}

// Chat sends a chat completion request to the GitHub Copilot API.
func (p *CopilotProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert messages to Copilot format
	messages := make([]copilotMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = copilotMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			Name:       msg.Name,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			messages[i].ToolCalls = make([]copilotToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				// Serialize arguments back to JSON string
				argsJSON, err := json.Marshal(tc.Arguments)
				if err != nil {
					argsJSON = []byte("{}")
				}
				messages[i].ToolCalls[j] = copilotToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: copilotFunctionCall{
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
		model = p.model
	}

	copilotReq := copilotRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	// Convert tools if present
	if req.Tools != nil {
		if tools, ok := req.Tools.([]map[string]interface{}); ok {
			copilotReq.Tools = tools
		} else if tools, ok := req.Tools.([]interface{}); ok {
			copilotReq.Tools = make([]map[string]interface{}, len(tools))
			for i, t := range tools {
				if toolMap, ok := t.(map[string]interface{}); ok {
					copilotReq.Tools[i] = toolMap
				}
			}
		}
	}

	// Marshal request body
	body, err := json.Marshal(copilotReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, CopilotAPIEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers for Copilot API
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.accessToken))
	httpReq.Header.Set("Copilot-Integration-Id", CopilotIntegrationID)

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
		return nil, fmt.Errorf("Copilot API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var copilotResp copilotResponse
	if err := json.Unmarshal(respBody, &copilotResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API-level errors
	if copilotResp.Error != nil {
		return nil, fmt.Errorf("Copilot API error: %s", copilotResp.Error.Message)
	}

	// Check for empty choices
	if len(copilotResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in Copilot response")
	}

	// Extract the first choice
	choice := copilotResp.Choices[0]

	// Build response
	chatResp := &ChatResponse{
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:     copilotResp.Usage.PromptTokens,
			CompletionTokens: copilotResp.Usage.CompletionTokens,
			TotalTokens:      copilotResp.Usage.TotalTokens,
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
