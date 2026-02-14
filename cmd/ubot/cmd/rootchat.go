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

	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/providers"
	"github.com/hkuds/ubot/internal/session"
	"github.com/hkuds/ubot/internal/skills"
	"github.com/hkuds/ubot/internal/tools"
	"github.com/spf13/cobra"
)

var rootchatCmd = &cobra.Command{
	Use:   "rootchat",
	Short: "Configure uBot interactively with elevated privileges",
	Long:  "Start an interactive chat session with elevated privileges to configure uBot. The assistant can read/write config, restart the gateway, and guide you through setup.",
	RunE:  runRootchat,
}

const rootchatSystemPrompt = `You are uBot's self-configuration assistant. You have elevated privileges to help the user set up and manage their uBot installation.

You can:
- Read and modify ~/.ubot/config.json using the manage_ubot tool
- Show the current configuration
- Restart the gateway after config changes
- Guide the user through setting up providers, channels, and tools

After making config changes, always suggest: "Config updated. Would you like me to restart the gateway to apply changes?"

## uBot Configuration Schema

The config file is located at ~/.ubot/config.json. Here is the full schema with all available fields:

### agents.defaults
- agents.defaults.workspace (string): Path to the agent workspace directory. Default: "~/.ubot/workspace"
- agents.defaults.model (string): LLM model to use. Default: "gpt-4". Examples: "gpt-4", "gpt-4o", "claude-3-opus-20240229", "anthropic/claude-sonnet-4-20250514", "google/gemini-pro"
- agents.defaults.maxTokens (int): Maximum tokens in LLM response. Default: 4096
- agents.defaults.temperature (float): Sampling temperature (0.0-2.0). Lower = more deterministic. Default: 0.7
- agents.defaults.maxToolIterations (int): Max number of tool call rounds per message. Default: 10

### providers
Configure at least one LLM provider. The first provider with a non-empty API key is used.
Priority order: copilot > openrouter > anthropic > openai > groq > gemini > vllm

- providers.openrouter.apiKey (string): OpenRouter API key
- providers.openrouter.apiBase (string): API base URL. Default: "https://openrouter.ai/api/v1"
- providers.anthropic.apiKey (string): Anthropic API key
- providers.anthropic.apiBase (string): API base URL. Default: "https://api.anthropic.com/v1"
- providers.openai.apiKey (string): OpenAI API key
- providers.openai.apiBase (string): API base URL. Default: "https://api.openai.com/v1"
- providers.groq.apiKey (string): Groq API key
- providers.groq.apiBase (string): API base URL. Default: "https://api.groq.com/openai/v1"
- providers.gemini.apiKey (string): Google Gemini API key
- providers.gemini.apiBase (string): API base URL. Default: "https://generativelanguage.googleapis.com/v1beta"
- providers.vllm.apiKey (string): vLLM API key (optional for local deployments)
- providers.vllm.apiBase (string): vLLM server URL. Default: "http://localhost:8000/v1"
- providers.copilot.enabled (bool): Enable GitHub Copilot provider. Default: false
- providers.copilot.accessToken (string): GitHub Copilot access token
- providers.copilot.model (string): Model to use with Copilot. Default: "gpt-4o"

### channels.telegram
- channels.telegram.enabled (bool): Enable Telegram channel. Default: false
- channels.telegram.token (string): Telegram bot token from @BotFather
- channels.telegram.allowFrom ([]string): Allowed Telegram usernames (without @). Empty = allow all

### channels.whatsapp
- channels.whatsapp.enabled (bool): Enable WhatsApp channel. Default: false
- channels.whatsapp.bridgeUrl (string): WhatsApp bridge URL. Default: "http://localhost:8080"
- channels.whatsapp.allowFrom ([]string): Allowed WhatsApp numbers. Empty = allow all

### gateway
- gateway.host (string): HTTP gateway bind address. Default: "127.0.0.1"
- gateway.port (int): HTTP gateway port. Default: 8080

### tools.web.search
- tools.web.search.apiKey (string): Web search API key (for web_search tool)
- tools.web.search.maxResults (int): Max search results to return. Default: 10

### tools.exec
- tools.exec.timeout (int): Shell command timeout in seconds. Default: 30
- tools.exec.restrictToWorkspace (bool): Restrict exec to workspace directory. Default: true

### tools.voice
- tools.voice.backend (string): Voice transcription backend: "groq" or "openai". Default: "groq" when Groq key is set
- tools.voice.model (string): Override default transcription model

### mcp.servers (array)
MCP (Model Context Protocol) server configurations. Each entry:
- mcp.servers[].name (string): Server display name
- mcp.servers[].command (string): Command to run (for stdio transport)
- mcp.servers[].args ([]string): Command arguments
- mcp.servers[].url (string): Server URL (for HTTP transport)
- mcp.servers[].transport (string): "stdio" or "http"
- mcp.servers[].env (map): Environment variables for the server process

## Common Tasks

1. **Set up a provider**: Use update_config to set the API key, e.g. key="providers.openrouter.apiKey" value="sk-..."
2. **Change model**: Use update_config with key="agents.defaults.model" value="claude-sonnet-4-20250514"
3. **Enable Telegram**: Set channels.telegram.enabled to "true" and channels.telegram.token to the bot token
4. **View current config**: Use show_config action
5. **Restart after changes**: Use restart action

Be concise and helpful. Guide the user step by step. Always show what you changed and offer to restart.`

func runRootchat(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create provider — for rootchat we still need an LLM, so check if one is configured
	providerName, _, _ := cfg.GetActiveProvider()
	if providerName == "" {
		fmt.Println("No LLM provider configured yet.")
		fmt.Println("Run 'ubot setup' first to configure at least one provider,")
		fmt.Println("then use 'ubot rootchat' to fine-tune your configuration.")
		return nil
	}

	provider, err := providers.NewProviderFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create session manager — rootchat uses its own session namespace
	dataDir := cfg.WorkspacePath()
	sessionMgr := session.NewManager(dataDir)
	sess := sessionMgr.GetOrCreate("cli:rootchat")

	// Create skills loader
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

	// Register manage_ubot tool with CLI source (elevated privileges)
	manageUbotTool := tools.NewManageUbotTool("")
	manageUbotTool.SetSource("cli")
	registry.Register(manageUbotTool)

	// Register browser tool
	browserTool := tools.NewBrowserTool(cfg.Tools.Browser)
	registry.Register(browserTool)

	// Wrap registry with security middleware
	secureReg := tools.NewSecureRegistry(registry)

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

	// Always interactive for rootchat
	return runRootchatInteractive(ctx, provider, sess, sessionMgr, secureReg, cfg, skillsSummary)
}

func runRootchatInteractive(ctx context.Context, provider providers.Provider, sess *session.Session, sessionMgr *session.Manager, registry *tools.SecureRegistry, cfg *config.Config, skillsSummary string) error {
	fmt.Println("uBot Root Configuration Mode")
	fmt.Println("I can help you configure providers, channels, models, and other settings.")
	fmt.Println("Type 'exit' or 'quit' to leave. Type '/clear' to reset history.")
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
		}

		err := sendRootchatMessage(ctx, provider, sess, sessionMgr, registry, cfg, input, skillsSummary)
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

func sendRootchatMessage(ctx context.Context, provider providers.Provider, sess *session.Session, sessionMgr *session.Manager, registry *tools.SecureRegistry, cfg *config.Config, message string, skillsSummary string) error {
	sess.AddMessage("user", message)

	messages := buildRootchatMessages(sess, skillsSummary)

	req := providers.ChatRequest{
		Messages:    messages,
		Tools:       registry.GetDefinitions(),
		Model:       cfg.Agents.Defaults.Model,
		MaxTokens:   cfg.Agents.Defaults.MaxTokens,
		Temperature: cfg.Agents.Defaults.Temperature,
	}

	response, err := provider.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("chat request failed: %w", err)
	}

	// Handle tool calls
	for response.HasToolCalls() {
		for _, toolCall := range response.ToolCalls {
			result, execErr := registry.Execute(ctx, toolCall.Name, toolCall.Arguments)
			if execErr != nil {
				result = fmt.Sprintf("Error: %v", execErr)
			}

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

		req.Messages = messages
		response, err = provider.Chat(ctx, req)
		if err != nil {
			return fmt.Errorf("chat request failed: %w", err)
		}
	}

	sess.AddMessage("assistant", response.Content)

	if err := sessionMgr.Save(sess); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	fmt.Println(response.Content)

	return nil
}

func buildRootchatMessages(sess *session.Session, skillsSummary string) []providers.ChatMessage {
	messages := sess.GetMessages()
	chatMessages := make([]providers.ChatMessage, 0, len(messages)+1)

	systemContent := rootchatSystemPrompt

	if skillsSummary != "" {
		systemContent += "\n\n" + skillsSummary
	}

	chatMessages = append(chatMessages, providers.ChatMessage{
		Role:    "system",
		Content: systemContent,
	})

	for _, msg := range messages {
		chatMessages = append(chatMessages, providers.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return chatMessages
}
