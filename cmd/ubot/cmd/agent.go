package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/providers"
	"github.com/hkuds/ubot/internal/session"
	"github.com/hkuds/ubot/internal/skills"
	"github.com/hkuds/ubot/internal/tools"
	"github.com/spf13/cobra"
)

var (
	messageFlag string
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Chat with the agent",
	Long:  "Start an interactive chat session with the agent, or send a single message.",
	RunE:  runAgent,
}

func init() {
	agentCmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Send a single message and exit")
}

func runAgent(cmd *cobra.Command, args []string) error {
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

	// Create provider
	provider, err := providers.NewProviderFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create session manager using the workspace directory
	dataDir := cfg.WorkspacePath()
	sessionMgr := session.NewManager(dataDir)

	// Get or create CLI session
	sess := sessionMgr.GetOrCreate("cli:default")

	// Create skills loader and discover available skills
	skillsLoader := skills.NewLoader(dataDir)
	if err := skillsLoader.Discover(); err != nil {
		log.Printf("Warning: failed to discover skills: %v", err)
	}
	skillsSummary := skillsLoader.GetSummary()

	// Create tool registry with default tools
	registry := tools.NewRegistry()
	registerDefaultTools(registry, cfg)

	// Register skill tools
	registerSkillTools(registry, skillsLoader)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nGoodbye!")
		cancel()
	}()

	// If message flag is provided, send single message and exit
	if messageFlag != "" {
		return sendSingleMessage(ctx, provider, sess, sessionMgr, registry, cfg, messageFlag, skillsSummary)
	}

	// Start interactive mode
	return runInteractiveMode(ctx, provider, sess, sessionMgr, registry, cfg, skillsSummary)
}

func sendSingleMessage(ctx context.Context, provider providers.Provider, sess *session.Session, sessionMgr *session.Manager, registry *tools.ToolRegistry, cfg *config.Config, message string, skillsSummary string) error {
	// Add user message to session
	sess.AddMessage("user", message)

	// Build messages for the LLM
	messages := buildChatMessages(sess, skillsSummary)

	// Create chat request
	req := providers.ChatRequest{
		Messages:    messages,
		Tools:       registry.GetDefinitions(),
		Model:       cfg.Agents.Defaults.Model,
		MaxTokens:   cfg.Agents.Defaults.MaxTokens,
		Temperature: cfg.Agents.Defaults.Temperature,
	}

	// Send request to LLM
	response, err := provider.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("chat request failed: %w", err)
	}

	// Handle tool calls if any
	for response.HasToolCalls() {
		// Execute tool calls
		for _, toolCall := range response.ToolCalls {
			result, err := registry.Execute(ctx, toolCall.Name, toolCall.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			// Add tool call and result to messages
			messages = append(messages, providers.ChatMessage{
				Role:      "assistant",
				Content:   response.Content,
				ToolCalls: response.ToolCalls,
			})
			messages = append(messages, providers.ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
				Name:       toolCall.Name,
			})
		}

		// Continue the conversation
		req.Messages = messages
		response, err = provider.Chat(ctx, req)
		if err != nil {
			return fmt.Errorf("chat request failed: %w", err)
		}
	}

	// Add assistant response to session
	sess.AddMessage("assistant", response.Content)

	// Save session
	if err := sessionMgr.Save(sess); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Print response
	fmt.Println(response.Content)

	return nil
}

func runInteractiveMode(ctx context.Context, provider providers.Provider, sess *session.Session, sessionMgr *session.Manager, registry *tools.ToolRegistry, cfg *config.Config, skillsSummary string) error {
	fmt.Println("uBot Interactive Mode")
	fmt.Println("Type your message and press Enter. Type 'exit' or 'quit' to leave.")
	fmt.Println("Commands: /clear (clear history), /help (show help)")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle special commands
		switch strings.ToLower(input) {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return nil
		case "/clear":
			sess.Clear()
			if err := sessionMgr.Save(sess); err != nil {
				fmt.Printf("Warning: failed to save session: %v\n", err)
			}
			fmt.Println("Conversation history cleared.")
			continue
		case "/help":
			printHelp()
			continue
		}

		// Send message and get response
		err := sendSingleMessage(ctx, provider, sess, sessionMgr, registry, cfg, input, skillsSummary)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			fmt.Printf("Error: %v\n", err)
		}
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("input error: %w", err)
	}

	return nil
}

func buildChatMessages(sess *session.Session, skillsSummary string) []providers.ChatMessage {
	messages := sess.GetMessages()
	chatMessages := make([]providers.ChatMessage, 0, len(messages)+1)

	// Build system message with optional skills summary
	systemContent := "You are uBot, a helpful AI assistant. You can use tools to help accomplish tasks. Be concise and helpful."

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
		chatMsg := providers.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		chatMessages = append(chatMessages, chatMsg)
	}

	return chatMessages
}

func registerDefaultTools(registry *tools.ToolRegistry, cfg *config.Config) {
	// Register filesystem tools
	readFile := tools.NewReadFileTool()
	writeFile := tools.NewWriteFileTool()
	listDir := tools.NewListDirTool()

	registry.Register(readFile)
	registry.Register(writeFile)
	registry.Register(listDir)

	// Register exec tool
	timeout := time.Duration(cfg.Tools.Exec.Timeout) * time.Second
	execTool := tools.NewExecToolWithOptions(timeout, cfg.WorkspacePath(), cfg.Tools.Exec.RestrictToWorkspace)
	registry.Register(execTool)

	// Register web tools if configured
	if cfg.Tools.Web.Search.APIKey != "" {
		searchTool := tools.NewWebSearchTool(cfg.Tools.Web.Search.APIKey, cfg.Tools.Web.Search.MaxResults)
		registry.Register(searchTool)
	}

	fetchTool := tools.NewWebFetchTool(50000) // 50KB max content
	registry.Register(fetchTool)
}

func printHelp() {
	fmt.Println()
	fmt.Println("uBot Interactive Mode Commands:")
	fmt.Println("  /clear    - Clear conversation history")
	fmt.Println("  /help     - Show this help message")
	fmt.Println("  exit/quit - Exit the chat")
	fmt.Println()
	fmt.Println("Available Tools:")
	fmt.Println("  - read_file: Read file contents")
	fmt.Println("  - write_file: Write content to a file")
	fmt.Println("  - list_dir: List directory contents")
	fmt.Println("  - exec: Execute shell commands")
	fmt.Println("  - web_search: Search the web (if configured)")
	fmt.Println("  - web_fetch: Fetch content from URLs")
	fmt.Println("  - list_skills: List available skills")
	fmt.Println("  - read_skill: Load a specific skill")
	fmt.Println()
}
