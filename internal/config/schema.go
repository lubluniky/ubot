package config

import (
	"os"
	"path/filepath"
)

// Config represents the root configuration structure for uBot.
type Config struct {
	Agents    AgentsConfig    `json:"agents"`
	Channels  ChannelsConfig  `json:"channels"`
	Providers ProvidersConfig `json:"providers"`
	Gateway   GatewayConfig   `json:"gateway"`
	Tools     ToolsConfig     `json:"tools"`
	MCP       MCPConfig       `json:"mcp"`
}

// AgentsConfig holds agent-related configuration with defaults.
type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults"`
}

// AgentDefaults defines default values for agent configuration.
type AgentDefaults struct {
	Workspace         string  `json:"workspace"`
	Model             string  `json:"model"`
	MaxTokens         int     `json:"maxTokens"`
	Temperature       float64 `json:"temperature"`
	MaxToolIterations int     `json:"maxToolIterations"`
}

// ChannelsConfig holds all communication channel configurations.
type ChannelsConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	WhatsApp WhatsAppConfig `json:"whatsapp"`
}

// TelegramConfig represents Telegram bot configuration.
type TelegramConfig struct {
	Enabled   bool     `json:"enabled"`
	Token     string   `json:"token"`
	AllowFrom []string `json:"allowFrom"`
}

// WhatsAppConfig represents WhatsApp bridge configuration.
type WhatsAppConfig struct {
	Enabled   bool     `json:"enabled"`
	BridgeURL string   `json:"bridgeUrl"`
	AllowFrom []string `json:"allowFrom"`
}

// ProvidersConfig holds all LLM provider configurations.
type ProvidersConfig struct {
	OpenRouter ProviderConfig        `json:"openrouter"`
	Anthropic  ProviderConfig        `json:"anthropic"`
	OpenAI     ProviderConfig        `json:"openai"`
	Groq       ProviderConfig        `json:"groq"`
	Gemini     ProviderConfig        `json:"gemini"`
	VLLM       ProviderConfig        `json:"vllm"`
	Copilot    CopilotProviderConfig `json:"copilot"`
	MiniMax    MiniMaxProviderConfig `json:"minimax"`
}

// ProviderConfig represents a standard LLM provider configuration.
type ProviderConfig struct {
	APIKey  string `json:"apiKey"`
	APIBase string `json:"apiBase,omitempty"`
}

// CopilotProviderConfig represents GitHub Copilot provider configuration.
type CopilotProviderConfig struct {
	Enabled     bool   `json:"enabled"`
	AccessToken string `json:"accessToken,omitempty"`
	Model       string `json:"model,omitempty"`
}

// MiniMaxProviderConfig represents MiniMax Coding Plan provider configuration.
type MiniMaxProviderConfig struct {
	Enabled bool   `json:"enabled"`
	Region  string `json:"region,omitempty"` // "global" or "cn"
	APIKey  string `json:"apiKey,omitempty"`
	Model   string `json:"model,omitempty"`
}

// GatewayConfig holds HTTP gateway configuration.
type GatewayConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// VoiceConfig holds voice transcription configuration.
type VoiceConfig struct {
	// Backend selects the transcription service: "groq" or "openai".
	// If empty, defaults to "groq" when a Groq API key is available.
	Backend string `json:"backend,omitempty"`
	// Model overrides the default model for the chosen backend.
	Model string `json:"model,omitempty"`
}

// ToolsConfig holds tool-related configurations.
type ToolsConfig struct {
	Web     WebToolsConfig `json:"web"`
	Exec    ExecToolConfig `json:"exec"`
	Voice   VoiceConfig    `json:"voice"`
	Browser BrowserConfig  `json:"browser"`
}

// BrowserConfig holds headless browser tool configuration.
type BrowserConfig struct {
	SessionDir  string `json:"sessionDir,omitempty"`  // persistent session storage dir; default ~/.ubot/workspace/browser-sessions
	Proxy       string `json:"proxy,omitempty"`       // proxy URL, e.g. "socks5://127.0.0.1:1080"
	Stealth     bool   `json:"stealth"`               // enable anti-detection stealth; default true
	IdleTimeout int    `json:"idleTimeout,omitempty"` // seconds before idle browser is closed; default 300
}

// WebToolsConfig represents web-related tools configuration.
type WebToolsConfig struct {
	Search WebSearchConfig `json:"search"`
}

// WebSearchConfig represents web search tool configuration.
type WebSearchConfig struct {
	APIKey     string `json:"apiKey"`
	MaxResults int    `json:"maxResults"`
}

// ExecToolConfig represents shell execution tool configuration.
type ExecToolConfig struct {
	Timeout             int  `json:"timeout"`
	RestrictToWorkspace bool `json:"restrictToWorkspace"`
}

// MCPConfig holds Model Context Protocol server configurations.
type MCPConfig struct {
	Servers []MCPServerConfig `json:"servers"`
}

// MCPServerConfig represents an MCP server configuration.
type MCPServerConfig struct {
	Name      string            `json:"name"`
	Command   string            `json:"command"`   // For stdio: command to run
	Args      []string          `json:"args"`      // Command arguments
	URL       string            `json:"url"`       // For HTTP: server URL
	Transport string            `json:"transport"` // "stdio" or "http"
	Env       map[string]string `json:"env"`       // Environment variables
}

// DefaultConfig returns a new Config with sensible default values.
func DefaultConfig() *Config {
	return &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:         "~/.ubot/workspace",
				Model:             "gpt-4",
				MaxTokens:         4096,
				Temperature:       0.7,
				MaxToolIterations: 10,
			},
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				Enabled:   false,
				Token:     "",
				AllowFrom: []string{},
			},
			WhatsApp: WhatsAppConfig{
				Enabled:   false,
				BridgeURL: "http://localhost:8080",
				AllowFrom: []string{},
			},
		},
		Providers: ProvidersConfig{
			OpenRouter: ProviderConfig{
				APIKey:  "",
				APIBase: "https://openrouter.ai/api/v1",
			},
			Anthropic: ProviderConfig{
				APIKey:  "",
				APIBase: "https://api.anthropic.com/v1",
			},
			OpenAI: ProviderConfig{
				APIKey:  "",
				APIBase: "https://api.openai.com/v1",
			},
			Groq: ProviderConfig{
				APIKey:  "",
				APIBase: "https://api.groq.com/openai/v1",
			},
			Gemini: ProviderConfig{
				APIKey:  "",
				APIBase: "https://generativelanguage.googleapis.com/v1beta",
			},
			VLLM: ProviderConfig{
				APIKey:  "",
				APIBase: "http://localhost:8000/v1",
			},
			Copilot: CopilotProviderConfig{
				Enabled:     false,
				AccessToken: "",
				Model:       "gpt-4o",
			},
			MiniMax: MiniMaxProviderConfig{
				Enabled: false,
				Region:  "global",
				Model:   "MiniMax-M2.5",
			},
		},
		Gateway: GatewayConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Tools: ToolsConfig{
			Web: WebToolsConfig{
				Search: WebSearchConfig{
					APIKey:     "",
					MaxResults: 10,
				},
			},
			Exec: ExecToolConfig{
				Timeout:             30,
				RestrictToWorkspace: true,
			},
			Browser: BrowserConfig{
				SessionDir:  "~/.ubot/workspace/browser-sessions",
				Stealth:     true,
				IdleTimeout: 300,
			},
		},
		MCP: MCPConfig{
			Servers: []MCPServerConfig{},
		},
	}
}

// WorkspacePath returns the absolute path to the workspace directory,
// expanding ~ to the user's home directory.
func (c *Config) WorkspacePath() string {
	workspace := c.Agents.Defaults.Workspace
	if workspace == "" {
		workspace = "~/.ubot/workspace"
	}
	return expandPath(workspace)
}

// GetActiveProvider returns the first configured provider's name, API key, and API base URL.
// It checks providers in order: Copilot, OpenRouter, Anthropic, OpenAI, Groq, Gemini, VLLM.
// Returns empty strings if no provider is configured.
func (c *Config) GetActiveProvider() (name string, apiKey string, apiBase string) {
	// Check Copilot first (uses access token instead of API key)
	if c.Providers.Copilot.Enabled && c.Providers.Copilot.AccessToken != "" {
		return "copilot", c.Providers.Copilot.AccessToken, ""
	}

	// Check MiniMax
	if c.Providers.MiniMax.Enabled && c.Providers.MiniMax.APIKey != "" {
		return "minimax", c.Providers.MiniMax.APIKey, ""
	}

	// Check OpenRouter
	if c.Providers.OpenRouter.APIKey != "" {
		return "openrouter", c.Providers.OpenRouter.APIKey, c.Providers.OpenRouter.APIBase
	}

	// Check Anthropic
	if c.Providers.Anthropic.APIKey != "" {
		return "anthropic", c.Providers.Anthropic.APIKey, c.Providers.Anthropic.APIBase
	}

	// Check OpenAI
	if c.Providers.OpenAI.APIKey != "" {
		return "openai", c.Providers.OpenAI.APIKey, c.Providers.OpenAI.APIBase
	}

	// Check Groq
	if c.Providers.Groq.APIKey != "" {
		return "groq", c.Providers.Groq.APIKey, c.Providers.Groq.APIBase
	}

	// Check Gemini
	if c.Providers.Gemini.APIKey != "" {
		return "gemini", c.Providers.Gemini.APIKey, c.Providers.Gemini.APIBase
	}

	// Check VLLM (may work without API key for local deployments)
	if c.Providers.VLLM.APIBase != "" {
		return "vllm", c.Providers.VLLM.APIKey, c.Providers.VLLM.APIBase
	}

	return "", "", ""
}

// expandPath expands ~ to the user's home directory and resolves the path.
func expandPath(path string) string {
	if path == "" {
		return path
	}

	// Expand ~ to home directory
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if len(path) == 1 {
			return home
		}
		// Handle ~/path and ~path cases
		if path[1] == '/' || path[1] == filepath.Separator {
			path = filepath.Join(home, path[2:])
		} else {
			path = filepath.Join(home, path[1:])
		}
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return absPath
}
