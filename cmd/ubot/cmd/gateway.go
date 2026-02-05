package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hkuds/ubot/internal/bus"
	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/providers"
	"github.com/hkuds/ubot/internal/session"
	"github.com/hkuds/ubot/internal/tools"
	"github.com/spf13/cobra"
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the channel gateway",
	Long:  "Start the gateway server that connects to configured channels (Telegram, WhatsApp) and processes messages.",
	RunE:  runGateway,
}

func runGateway(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if provider is configured
	providerName, _, _ := cfg.GetActiveProvider()
	if providerName == "" {
		fmt.Println("No LLM provider configured.")
		fmt.Println("Run 'ubot setup' to configure a provider.")
		return nil
	}

	// Check if any channel is enabled
	if !cfg.Channels.Telegram.Enabled && !cfg.Channels.WhatsApp.Enabled {
		fmt.Println("No channels configured.")
		fmt.Println("Run 'ubot setup' to configure Telegram or WhatsApp.")
		return nil
	}

	// Create message bus
	msgBus := bus.NewMessageBus(100)
	defer msgBus.Close()

	// Create provider
	provider, err := providers.NewProviderFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create session manager using the workspace directory
	dataDir := cfg.WorkspacePath()
	sessionMgr := session.NewManager(dataDir)

	// Create tool registry with default tools
	registry := tools.NewRegistry()
	registerDefaultTools(registry, cfg)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start outbound message dispatcher
	go msgBus.DispatchOutbound(ctx)

	// WaitGroup for tracking running goroutines
	var wg sync.WaitGroup

	// Start agent loop (processes inbound messages)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runAgentLoop(ctx, msgBus, provider, sessionMgr, registry, cfg)
	}()

	// Start channel connectors
	if cfg.Channels.Telegram.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runTelegramChannel(ctx, msgBus, cfg)
		}()
		fmt.Printf("Telegram channel: enabled\n")
	}

	if cfg.Channels.WhatsApp.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runWhatsAppChannel(ctx, msgBus, cfg)
		}()
		fmt.Printf("WhatsApp channel: enabled\n")
	}

	fmt.Printf("Provider: %s (model: %s)\n", providerName, cfg.Agents.Defaults.Model)
	fmt.Println()
	fmt.Println("Gateway is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down gateway...")

	// Cancel context to stop all goroutines
	cancel()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("Gateway stopped gracefully.")
	case <-time.After(10 * time.Second):
		fmt.Println("Gateway shutdown timed out.")
	}

	return nil
}

// runAgentLoop processes inbound messages and sends responses.
func runAgentLoop(ctx context.Context, msgBus *bus.MessageBus, provider providers.Provider, sessionMgr *session.Manager, registry *tools.ToolRegistry, cfg *config.Config) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Wait for inbound message with timeout
		msg, err := msgBus.ConsumeInboundWithTimeout(ctx, 1*time.Second)
		if err != nil {
			if err == bus.ErrTimeout {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			continue
		}

		// Process message in a goroutine
		go processMessage(ctx, msgBus, provider, sessionMgr, registry, cfg, msg)
	}
}

// processMessage handles a single inbound message.
func processMessage(ctx context.Context, msgBus *bus.MessageBus, provider providers.Provider, sessionMgr *session.Manager, registry *tools.ToolRegistry, cfg *config.Config, msg bus.InboundMessage) {
	// Get or create session for this conversation
	sess := sessionMgr.GetOrCreate(msg.SessionKey())

	// Add user message to session
	sess.AddMessage("user", msg.Content)

	// Build messages for the LLM
	messages := buildChatMessagesFromSession(sess)

	// Create chat request
	req := providers.ChatRequest{
		Messages:    messages,
		Tools:       registry.GetDefinitions(),
		Model:       cfg.Agents.Defaults.Model,
		MaxTokens:   cfg.Agents.Defaults.MaxTokens,
		Temperature: cfg.Agents.Defaults.Temperature,
	}

	// Iterate through tool calls up to max iterations
	iterations := 0
	maxIterations := cfg.Agents.Defaults.MaxToolIterations

	for iterations < maxIterations {
		// Send request to LLM
		response, err := provider.Chat(ctx, req)
		if err != nil {
			fmt.Printf("Error from provider: %v\n", err)
			sendErrorResponse(msgBus, msg, "I encountered an error processing your request.")
			return
		}

		// If no tool calls, we have the final response
		if !response.HasToolCalls() {
			// Add assistant response to session
			sess.AddMessage("assistant", response.Content)

			// Save session
			if err := sessionMgr.Save(sess); err != nil {
				fmt.Printf("Warning: failed to save session: %v\n", err)
			}

			// Send response
			msgBus.PublishOutbound(bus.OutboundMessage{
				Channel: msg.Channel,
				ChatID:  msg.ChatID,
				Content: response.Content,
			})
			return
		}

		// Execute tool calls
		messages = append(messages, providers.ChatMessage{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		for _, toolCall := range response.ToolCalls {
			result, err := registry.Execute(ctx, toolCall.Name, toolCall.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error executing tool: %v", err)
			}

			messages = append(messages, providers.ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
				Name:       toolCall.Name,
			})
		}

		req.Messages = messages
		iterations++
	}

	// Max iterations reached
	sendErrorResponse(msgBus, msg, "I've reached the maximum number of tool iterations. Please try a simpler request.")
}

// buildChatMessagesFromSession converts session messages to chat messages.
func buildChatMessagesFromSession(sess *session.Session) []providers.ChatMessage {
	messages := sess.GetMessages()
	chatMessages := make([]providers.ChatMessage, 0, len(messages)+1)

	// Add system message
	chatMessages = append(chatMessages, providers.ChatMessage{
		Role:    "system",
		Content: "You are uBot, a helpful AI assistant. You can use tools to help accomplish tasks. Be concise and helpful.",
	})

	// Convert session messages to chat messages
	for _, msg := range messages {
		chatMessages = append(chatMessages, providers.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return chatMessages
}

// sendErrorResponse sends an error message back to the channel.
func sendErrorResponse(msgBus *bus.MessageBus, msg bus.InboundMessage, errorMsg string) {
	msgBus.PublishOutbound(bus.OutboundMessage{
		Channel: msg.Channel,
		ChatID:  msg.ChatID,
		Content: errorMsg,
	})
}

// runTelegramChannel starts the Telegram channel connector.
// This is a placeholder that will be implemented when the Telegram channel is added.
func runTelegramChannel(ctx context.Context, msgBus *bus.MessageBus, cfg *config.Config) {
	// TODO: Implement Telegram channel connector
	// For now, this is a placeholder that waits for context cancellation
	fmt.Println("Telegram channel connector started (placeholder)")
	<-ctx.Done()
}

// runWhatsAppChannel starts the WhatsApp channel connector.
// This is a placeholder that will be implemented when the WhatsApp bridge is added.
func runWhatsAppChannel(ctx context.Context, msgBus *bus.MessageBus, cfg *config.Config) {
	// TODO: Implement WhatsApp channel connector
	// For now, this is a placeholder that waits for context cancellation
	fmt.Println("WhatsApp channel connector started (placeholder)")
	<-ctx.Done()
}
