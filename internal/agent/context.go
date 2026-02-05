package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/providers"
	"github.com/hkuds/ubot/internal/session"
)

// ContextBuilder builds system prompts and message arrays for LLM interactions.
type ContextBuilder struct {
	workspace string
	config    *config.Config
	memory    *MemoryStore
}

// NewContextBuilder creates a new ContextBuilder with the given configuration.
func NewContextBuilder(cfg *config.Config) *ContextBuilder {
	workspace := cfg.WorkspacePath()
	return &ContextBuilder{
		workspace: workspace,
		config:    cfg,
		memory:    NewMemoryStore(workspace),
	}
}

// BuildSystemPrompt builds the system prompt from workspace files.
// It includes identity, date/time, workspace path, and content from
// AGENTS.md, SOUL.md, USER.md, TOOLS.md, and MEMORY.md if they exist.
func (c *ContextBuilder) BuildSystemPrompt() string {
	var parts []string

	// Identity
	parts = append(parts, "You are uBot, an AI assistant.")

	// Current date and time
	now := time.Now()
	parts = append(parts, fmt.Sprintf("Current date and time: %s", now.Format("2006-01-02 15:04:05 MST")))

	// Workspace path
	parts = append(parts, fmt.Sprintf("Workspace path: %s", c.workspace))

	// Load optional configuration files
	if content := c.loadWorkspaceFile("AGENTS.md"); content != "" {
		parts = append(parts, "\n## Agent Configuration\n"+content)
	}

	if content := c.loadWorkspaceFile("SOUL.md"); content != "" {
		parts = append(parts, "\n## Personality & Behavior\n"+content)
	}

	if content := c.loadWorkspaceFile("USER.md"); content != "" {
		parts = append(parts, "\n## User Context\n"+content)
	}

	if content := c.loadWorkspaceFile("TOOLS.md"); content != "" {
		parts = append(parts, "\n## Tool Usage Guidelines\n"+content)
	}

	// Load memory context
	if memoryContent := c.memory.GetMemoryContext(); memoryContent != "" {
		parts = append(parts, "\n## Memory\n"+memoryContent)
	}

	// Load today's notes
	if dailyNotes := c.memory.GetDailyNotes(); dailyNotes != "" {
		parts = append(parts, "\n## Today's Notes\n"+dailyNotes)
	}

	// Tool usage explanation
	parts = append(parts, `
## Available Tools

You have access to various tools that you can use to accomplish tasks:

- **read_file**: Read the contents of a file
- **write_file**: Write content to a file
- **list_directory**: List files and directories
- **shell**: Execute shell commands
- **web_search**: Search the web for information
- **web_fetch**: Fetch content from a URL
- **message**: Send a message to a channel or chat

When using tools:
1. Always explain what you're doing before using a tool
2. Handle errors gracefully and report them to the user
3. Use tools efficiently - combine operations when possible
4. Respect file system boundaries and security constraints`)

	return strings.Join(parts, "\n\n")
}

// loadWorkspaceFile loads a file from the workspace directory.
// Returns empty string if the file doesn't exist or cannot be read.
func (c *ContextBuilder) loadWorkspaceFile(filename string) string {
	filePath := filepath.Join(c.workspace, filename)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

// BuildMessages builds the full messages array for the LLM.
// It includes the system prompt, conversation history, and the current user message.
func (c *ContextBuilder) BuildMessages(history []session.Message, userContent string, media []string) []providers.ChatMessage {
	messages := make([]providers.ChatMessage, 0, len(history)+2)

	// Add system prompt
	messages = append(messages, providers.ChatMessage{
		Role:    "system",
		Content: c.BuildSystemPrompt(),
	})

	// Add conversation history
	// Note: The session.Message from manager.go only has Role and Content
	// Tool calls and their results are not persisted in history currently
	for _, msg := range history {
		chatMsg := providers.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		messages = append(messages, chatMsg)
	}

	// Add current user message
	if userContent != "" {
		userMsg := providers.ChatMessage{
			Role: "user",
		}

		// Handle media attachments (images, etc.)
		if len(media) > 0 {
			// Build multimodal content
			userMsg.Content = c.buildMultimodalContent(userContent, media)
		} else {
			userMsg.Content = userContent
		}

		messages = append(messages, userMsg)
	}

	return messages
}

// ContentPart represents a part of multimodal content.
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL in multimodal content.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// buildMultimodalContent builds content with text and images for vision models.
func (c *ContextBuilder) buildMultimodalContent(text string, media []string) interface{} {
	parts := make([]ContentPart, 0, len(media)+1)

	// Add text content first
	if text != "" {
		parts = append(parts, ContentPart{
			Type: "text",
			Text: text,
		})
	}

	// Add image URLs
	for _, url := range media {
		parts = append(parts, ContentPart{
			Type: "image_url",
			ImageURL: &ImageURL{
				URL:    url,
				Detail: "auto",
			},
		})
	}

	return parts
}

// AddAssistantMessage adds an assistant response with optional tool calls to the messages array.
func (c *ContextBuilder) AddAssistantMessage(messages []providers.ChatMessage, content string, toolCalls []providers.ToolCall) []providers.ChatMessage {
	msg := providers.ChatMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}
	return append(messages, msg)
}

// AddToolResult adds a tool execution result to the messages array.
func (c *ContextBuilder) AddToolResult(messages []providers.ChatMessage, toolCallID, name, result string) []providers.ChatMessage {
	msg := providers.ChatMessage{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
		Name:       name,
	}
	return append(messages, msg)
}


// GetWorkspace returns the workspace path.
func (c *ContextBuilder) GetWorkspace() string {
	return c.workspace
}

// GetMemory returns the memory store.
func (c *ContextBuilder) GetMemory() *MemoryStore {
	return c.memory
}
