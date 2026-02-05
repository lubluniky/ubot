package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hkuds/ubot/internal/bus"
	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/providers"
	"github.com/hkuds/ubot/internal/session"
	"github.com/hkuds/ubot/internal/tools"
)

// Loop is the main agent processing loop that handles incoming messages,
// coordinates with the LLM provider, executes tools, and manages sessions.
type Loop struct {
	bus      *bus.MessageBus
	provider providers.Provider
	sessions *session.Manager
	tools    *tools.ToolRegistry
	config   *config.Config

	messageTool *tools.MessageTool
	context     *ContextBuilder

	running bool
	mu      sync.RWMutex

	// stopCh is used to signal the loop to stop
	stopCh chan struct{}
}

// LoopConfig contains the configuration for creating a new Loop.
type LoopConfig struct {
	Bus      *bus.MessageBus
	Provider providers.Provider
	Config   *config.Config
	Sessions *session.Manager
}

// NewLoop creates a new agent loop with the given configuration.
func NewLoop(cfg LoopConfig) (*Loop, error) {
	if cfg.Bus == nil {
		return nil, fmt.Errorf("message bus is required")
	}
	if cfg.Provider == nil {
		return nil, fmt.Errorf("provider is required")
	}
	if cfg.Config == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Create session manager if not provided
	sessions := cfg.Sessions
	if sessions == nil {
		// Use the config's workspace path for session storage
		dataDir := cfg.Config.WorkspacePath()
		sessions = session.NewManager(dataDir)
	}

	// Create tool registry with default tools
	registry := tools.NewRegistry()

	// Create message tool with callback to publish to bus
	messageTool := tools.NewMessageTool(func(msg tools.OutboundMessage) error {
		cfg.Bus.PublishOutbound(bus.OutboundMessage{
			Channel: msg.Channel,
			ChatID:  msg.ChatID,
			Content: msg.Content,
		})
		return nil
	})

	// Register the message tool
	if err := registry.Register(messageTool); err != nil {
		return nil, fmt.Errorf("failed to register message tool: %w", err)
	}

	// Create context builder
	contextBuilder := NewContextBuilder(cfg.Config)

	return &Loop{
		bus:         cfg.Bus,
		provider:    cfg.Provider,
		sessions:    sessions,
		tools:       registry,
		config:      cfg.Config,
		messageTool: messageTool,
		context:     contextBuilder,
		stopCh:      make(chan struct{}),
	}, nil
}

// Run starts the agent loop and processes messages until the context is cancelled.
func (l *Loop) Run(ctx context.Context) error {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return fmt.Errorf("loop is already running")
	}
	l.running = true
	l.stopCh = make(chan struct{})
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		l.running = false
		l.mu.Unlock()
	}()

	// Start outbound message dispatcher
	go l.bus.DispatchOutbound(ctx)

	log.Println("Agent loop started")

	// Main processing loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Agent loop stopped: context cancelled")
			return ctx.Err()
		case <-l.stopCh:
			log.Println("Agent loop stopped: stop signal received")
			return nil
		default:
			// Try to consume an inbound message with timeout
			msg, err := l.bus.ConsumeInboundWithTimeout(ctx, 1*time.Second)
			if err == bus.ErrTimeout {
				continue
			}
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				log.Printf("Error consuming message: %v", err)
				continue
			}

			// Process the message
			response, err := l.ProcessMessage(ctx, msg)
			if err != nil {
				log.Printf("Error processing message: %v", err)
				// Send error message back
				l.bus.PublishOutbound(bus.OutboundMessage{
					Channel: msg.Channel,
					ChatID:  msg.ChatID,
					Content: fmt.Sprintf("Error: %v", err),
				})
				continue
			}

			// Publish the response if we got one
			if response != nil && response.Content != "" {
				l.bus.PublishOutbound(*response)
			}
		}
	}
}

// Stop signals the loop to stop processing.
func (l *Loop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		close(l.stopCh)
	}
}

// IsRunning returns whether the loop is currently running.
func (l *Loop) IsRunning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.running
}

// ProcessMessage processes a single inbound message and returns an outbound response.
func (l *Loop) ProcessMessage(ctx context.Context, msg bus.InboundMessage) (*bus.OutboundMessage, error) {
	// Handle system channel messages (subagent results) specially
	if msg.Channel == "system" {
		return l.handleSystemMessage(ctx, msg)
	}

	// Get or create session for this conversation
	sessionKey := msg.SessionKey()
	sess := l.sessions.GetOrCreate(sessionKey)

	// Update MessageTool context with current channel/chatID
	l.messageTool.SetContext(msg.Channel, msg.ChatID)
	defer l.messageTool.ClearContext()

	// Get conversation history
	history := sess.GetMessages() // Get all messages

	// Build messages array: system prompt + history + current user message
	messages := l.context.BuildMessages(history, msg.Content, msg.Media)

	// Get tool definitions
	toolDefs := l.tools.GetDefinitions()

	// Get max iterations from config
	maxIterations := l.config.Agents.Defaults.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}

	var finalContent string

	// Tool execution loop
	for i := 0; i < maxIterations; i++ {
		// Call LLM provider
		req := providers.ChatRequest{
			Messages:    messages,
			Tools:       toolDefs,
			Model:       l.config.Agents.Defaults.Model,
			MaxTokens:   l.config.Agents.Defaults.MaxTokens,
			Temperature: l.config.Agents.Defaults.Temperature,
		}

		resp, err := l.provider.Chat(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("LLM chat failed: %w", err)
		}

		// If no tool calls, we're done
		if !resp.HasToolCalls() {
			finalContent = resp.Content
			break
		}

		// Add assistant message with tool calls to messages
		messages = l.context.AddAssistantMessage(messages, resp.Content, resp.ToolCalls)

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			result, err := l.executeTool(ctx, tc)
			if err != nil {
				result = fmt.Sprintf("Tool error: %v", err)
			}

			// Add tool result to messages
			messages = l.context.AddToolResult(messages, tc.ID, tc.Name, result)
		}

		// If this is the last iteration, get a final response without tools
		if i == maxIterations-1 {
			req.Tools = nil
			resp, err := l.provider.Chat(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("final LLM chat failed: %w", err)
			}
			finalContent = resp.Content
		}
	}

	// Save user message and assistant response to session
	sess.AddMessage("user", msg.Content)
	if finalContent != "" {
		sess.AddMessage("assistant", finalContent)
	}

	// Save session
	if err := l.sessions.Save(sess); err != nil {
		log.Printf("Warning: failed to save session: %v", err)
	}

	// Return the outbound message
	return &bus.OutboundMessage{
		Channel: msg.Channel,
		ChatID:  msg.ChatID,
		Content: finalContent,
	}, nil
}

// executeTool executes a single tool call and returns the result.
func (l *Loop) executeTool(ctx context.Context, tc providers.ToolCall) (string, error) {
	// Execute the tool
	result, err := l.tools.Execute(ctx, tc.Name, tc.Arguments)
	if err != nil {
		return "", err
	}
	return result, nil
}

// handleSystemMessage handles messages from the "system" channel (e.g., subagent results).
func (l *Loop) handleSystemMessage(ctx context.Context, msg bus.InboundMessage) (*bus.OutboundMessage, error) {
	// System messages are typically results from subagents or internal events
	// For now, we just log them and don't send a response
	log.Printf("System message received: %s", msg.Content)

	// Check if there's a target session in metadata
	if msg.Metadata != nil {
		if targetSession, ok := msg.Metadata["targetSession"].(string); ok {
			// Route the result to the target session
			sess := l.sessions.Get(targetSession)
			if sess == nil {
				return nil, fmt.Errorf("failed to get target session: session not found")
			}

			// Add the system message to the session
			sess.AddMessage("system", msg.Content)

			if err := l.sessions.Save(sess); err != nil {
				log.Printf("Warning: failed to save session: %v", err)
			}
		}
	}

	// System messages don't generate outbound responses
	return nil, nil
}

// RegisterTool adds a new tool to the loop's registry.
func (l *Loop) RegisterTool(t tools.Tool) error {
	return l.tools.Register(t)
}

// UnregisterTool removes a tool from the loop's registry.
func (l *Loop) UnregisterTool(name string) {
	l.tools.Unregister(name)
}

// GetToolRegistry returns the tool registry.
func (l *Loop) GetToolRegistry() *tools.ToolRegistry {
	return l.tools
}

// GetSessionManager returns the session manager.
func (l *Loop) GetSessionManager() *session.Manager {
	return l.sessions
}

// GetProvider returns the LLM provider.
func (l *Loop) GetProvider() providers.Provider {
	return l.provider
}

// GetConfig returns the configuration.
func (l *Loop) GetConfig() *config.Config {
	return l.config
}

// GetContextBuilder returns the context builder.
func (l *Loop) GetContextBuilder() *ContextBuilder {
	return l.context
}

// SendMessage sends a message to a specific channel and chat.
// This is useful for proactive messages from the agent.
func (l *Loop) SendMessage(channel, chatID, content string) {
	l.bus.PublishOutbound(bus.OutboundMessage{
		Channel: channel,
		ChatID:  chatID,
		Content: content,
	})
}

// InjectMessage injects a message into the inbound queue for processing.
// This is useful for testing or internal message generation.
func (l *Loop) InjectMessage(msg bus.InboundMessage) {
	l.bus.PublishInbound(msg)
}

