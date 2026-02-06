package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hkuds/ubot/internal/bus"
	"github.com/hkuds/ubot/internal/channels"
	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/cron"
	"github.com/hkuds/ubot/internal/mcp"
	"github.com/hkuds/ubot/internal/providers"
	"github.com/hkuds/ubot/internal/session"
	"github.com/hkuds/ubot/internal/skills"
	"github.com/hkuds/ubot/internal/tools"
	"github.com/hkuds/ubot/internal/voice"
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

	// Create skills loader and discover available skills
	skillsLoader := skills.NewLoader(dataDir)
	bundledSkillsPath := config.GetConfigDir() + "/repo/skills"
	skillsLoader.SetBundledPath(bundledSkillsPath)
	if err := skillsLoader.Discover(); err != nil {
		log.Printf("Warning: failed to discover skills: %v", err)
	}
	skillsSummary := skillsLoader.GetSummary()

	// Create tool registry with default tools
	registry := tools.NewRegistry()
	registerDefaultTools(registry, cfg)

	// Register skill tools
	registerSkillTools(registry, skillsLoader)

	// Register manage_ubot tool
	manageUbotTool := tools.NewManageUbotTool("")
	registry.Register(manageUbotTool)

	// Register browser tool
	browserTool := tools.NewBrowserTool()
	registry.Register(browserTool)

	// Create and start proactive cron scheduler
	scheduler := cron.NewScheduler(msgBus, provider, cfg.Agents.Defaults.Model)
	cronTool := tools.NewCronTool(scheduler)
	registry.Register(cronTool)

	// Wrap registry with security middleware
	secureReg := tools.NewSecureRegistry(registry)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start proactive cron scheduler
	if err := scheduler.Start(ctx); err != nil {
		log.Printf("Warning: failed to start cron scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Initialize MCP manager and connect to configured servers
	mcpManager := mcp.NewManager()
	defer mcpManager.Close()
	registerMCPServers(ctx, mcpManager, cfg, registry)

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
		runAgentLoop(ctx, msgBus, provider, sessionMgr, secureReg, cfg, skillsSummary, manageUbotTool)
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
func runAgentLoop(ctx context.Context, msgBus *bus.MessageBus, provider providers.Provider, sessionMgr *session.Manager, registry *tools.SecureRegistry, cfg *config.Config, skillsSummary string, manageUbotTool *tools.ManageUbotTool) {
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
		go processMessage(ctx, msgBus, provider, sessionMgr, registry, cfg, msg, skillsSummary, manageUbotTool)
	}
}

// processMessage handles a single inbound message.
func processMessage(ctx context.Context, msgBus *bus.MessageBus, provider providers.Provider, sessionMgr *session.Manager, registry *tools.SecureRegistry, cfg *config.Config, msg bus.InboundMessage, skillsSummary string, manageUbotTool *tools.ManageUbotTool) {
	// Get or create session for this conversation
	sess := sessionMgr.GetOrCreate(msg.SessionKey())
	sess.Source = msg.Channel

	// Set manage_ubot tool source context for this request
	manageUbotTool.SetSource(msg.Channel)
	defer manageUbotTool.ClearSource()

	// Add user message to session
	sess.AddMessage("user", msg.Content)

	// Build messages for the LLM
	messages := buildChatMessagesFromSession(sess, skillsSummary)

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
func buildChatMessagesFromSession(sess *session.Session, skillsSummary string) []providers.ChatMessage {
	messages := sess.GetMessages()
	chatMessages := make([]providers.ChatMessage, 0, len(messages)+1)

	// Build system message with optional skills summary
	systemContent := `You are uBot — the world's most lightweight self-hosted AI assistant.

Key facts about yourself:
- Ultra-minimal: ~10,000 lines of Go code (compared to 400k+ lines in similar projects)
- Self-hosted: users run you on their own hardware, keeping data private
- Multi-channel: you work through Telegram, WhatsApp, and CLI
- Tool-capable: you can read/write files, execute commands, search the web
- Fast: compiled Go binary, instant startup, minimal memory footprint

Personality: Be helpful, concise, and technically competent. You're proud of being lightweight but not boastful. Answer in the user's language.`

	// Append skills summary if available
	if skillsSummary != "" {
		systemContent += "\n\n" + skillsSummary
	}

	// Add system message
	chatMessages = append(chatMessages, providers.ChatMessage{
		Role:    "system",
		Content: systemContent,
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
func runTelegramChannel(ctx context.Context, msgBus *bus.MessageBus, cfg *config.Config) {
	// Build voice transcriber (nil when not configured)
	transcriber := buildVoiceTranscriber(cfg)

	// Create the Telegram channel
	telegramChannel := channels.NewTelegramChannel(cfg.Channels.Telegram, msgBus, transcriber)

	// Start the channel
	if err := telegramChannel.Start(ctx); err != nil {
		log.Printf("Failed to start Telegram channel: %v", err)
		return
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Stop the channel gracefully
	if err := telegramChannel.Stop(); err != nil {
		log.Printf("Error stopping Telegram channel: %v", err)
	}
}

// runWhatsAppChannel starts the WhatsApp channel connector.
// This is a placeholder that will be implemented when the WhatsApp bridge is added.
func runWhatsAppChannel(ctx context.Context, msgBus *bus.MessageBus, cfg *config.Config) {
	// TODO: Implement WhatsApp channel connector
	// For now, this is a placeholder that waits for context cancellation
	fmt.Println("WhatsApp channel connector started (placeholder)")
	<-ctx.Done()
}

// registerSkillTools registers skill-related tools to the registry.
func registerSkillTools(registry *tools.ToolRegistry, loader *skills.Loader) {
	// Register read_skill tool
	readSkillTool := tools.NewReadSkillTool(loader)
	registry.Register(readSkillTool)

	// Register list_skills tool
	listSkillsTool := tools.NewListSkillsTool(loader)
	registry.Register(listSkillsTool)
}

// registerMCPServers connects to configured MCP servers and registers their tools.
func registerMCPServers(ctx context.Context, manager *mcp.Manager, cfg *config.Config, registry *tools.ToolRegistry) {
	if len(cfg.MCP.Servers) == 0 {
		return
	}

	fmt.Printf("Connecting to MCP servers...\n")

	for _, serverCfg := range cfg.MCP.Servers {
		// Convert config server to mcp.Server
		server := mcp.Server{
			Name:      serverCfg.Name,
			Command:   serverCfg.Command,
			Args:      serverCfg.Args,
			URL:       serverCfg.URL,
			Transport: serverCfg.Transport,
			Env:       serverCfg.Env,
		}

		// Connect to the server
		if err := manager.AddServer(ctx, server); err != nil {
			log.Printf("Warning: failed to connect to MCP server %q: %v", serverCfg.Name, err)
			continue
		}

		fmt.Printf("MCP server %q: connected\n", serverCfg.Name)
	}

	// Register all MCP tools with the tool registry
	bridgedTools := manager.CreateBridgedTools()
	for _, tool := range bridgedTools {
		if err := registry.Register(tool); err != nil {
			log.Printf("Warning: failed to register MCP tool %q: %v", tool.Name(), err)
		}
	}

	if len(bridgedTools) > 0 {
		fmt.Printf("MCP tools registered: %d\n", len(bridgedTools))
	}
}

// buildVoiceTranscriber creates a voice.Transcriber based on config.
// Returns nil when no suitable API key is available.
func buildVoiceTranscriber(cfg *config.Config) *voice.Transcriber {
	voiceCfg := cfg.Tools.Voice

	backend := voice.Backend(voiceCfg.Backend)
	var apiKey string

	switch backend {
	case voice.BackendOpenAI:
		apiKey = cfg.Providers.OpenAI.APIKey
	case voice.BackendGroq:
		apiKey = cfg.Providers.Groq.APIKey
	default:
		if cfg.Providers.Groq.APIKey != "" {
			backend = voice.BackendGroq
			apiKey = cfg.Providers.Groq.APIKey
		} else if cfg.Providers.OpenAI.APIKey != "" {
			backend = voice.BackendOpenAI
			apiKey = cfg.Providers.OpenAI.APIKey
		}
	}

	if apiKey == "" {
		return nil
	}

	var opts []voice.Option
	if voiceCfg.Model != "" {
		opts = append(opts, voice.WithModel(voiceCfg.Model))
	}

	t, err := voice.NewTranscriber(backend, apiKey, opts...)
	if err != nil {
		log.Printf("Failed to create voice transcriber: %v", err)
		return nil
	}
	return t
}
