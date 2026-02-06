package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Agents.Defaults.Model != "gpt-4" {
		t.Errorf("default model = %q, want %q", cfg.Agents.Defaults.Model, "gpt-4")
	}
	if cfg.Agents.Defaults.MaxTokens != 4096 {
		t.Errorf("default maxTokens = %d, want 4096", cfg.Agents.Defaults.MaxTokens)
	}
	if cfg.Agents.Defaults.Temperature != 0.7 {
		t.Errorf("default temperature = %f, want 0.7", cfg.Agents.Defaults.Temperature)
	}
	if cfg.Agents.Defaults.MaxToolIterations != 10 {
		t.Errorf("default maxToolIterations = %d, want 10", cfg.Agents.Defaults.MaxToolIterations)
	}
	if cfg.Gateway.Port != 8080 {
		t.Errorf("default port = %d, want 8080", cfg.Gateway.Port)
	}
	if cfg.Channels.Telegram.Enabled {
		t.Error("telegram should be disabled by default")
	}
	if cfg.Channels.WhatsApp.Enabled {
		t.Error("whatsapp should be disabled by default")
	}
	if cfg.Tools.Exec.Timeout != 30 {
		t.Errorf("default exec timeout = %d, want 30", cfg.Tools.Exec.Timeout)
	}
	if !cfg.Tools.Exec.RestrictToWorkspace {
		t.Error("restrictToWorkspace should be true by default")
	}
}

func TestWorkspacePath(t *testing.T) {
	cfg := DefaultConfig()
	path := cfg.WorkspacePath()

	if path == "" {
		t.Error("WorkspacePath() should not be empty")
	}
	if path == "~/.ubot/workspace" {
		t.Error("WorkspacePath() should expand tilde")
	}
}

func TestWorkspacePathEmpty(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = ""
	path := cfg.WorkspacePath()

	if path == "" {
		t.Error("WorkspacePath() should use default when empty")
	}
}

func TestGetActiveProvider(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		wantName string
	}{
		{
			name: "no providers configured",
			cfg: func() *Config {
				c := DefaultConfig()
				// DefaultConfig sets VLLM APIBase, clear it
				c.Providers.VLLM.APIBase = ""
				return c
			}(),
			wantName: "",
		},
		{
			name: "copilot enabled",
			cfg: func() *Config {
				c := DefaultConfig()
				c.Providers.Copilot.Enabled = true
				c.Providers.Copilot.AccessToken = "token"
				return c
			}(),
			wantName: "copilot",
		},
		{
			name: "openrouter configured",
			cfg: func() *Config {
				c := DefaultConfig()
				c.Providers.OpenRouter.APIKey = "sk-test"
				return c
			}(),
			wantName: "openrouter",
		},
		{
			name: "anthropic configured",
			cfg: func() *Config {
				c := DefaultConfig()
				c.Providers.Anthropic.APIKey = "sk-ant-test"
				return c
			}(),
			wantName: "anthropic",
		},
		{
			name: "openai configured",
			cfg: func() *Config {
				c := DefaultConfig()
				c.Providers.OpenAI.APIKey = "sk-test"
				return c
			}(),
			wantName: "openai",
		},
		{
			name: "groq configured",
			cfg: func() *Config {
				c := DefaultConfig()
				c.Providers.Groq.APIKey = "gsk-test"
				return c
			}(),
			wantName: "groq",
		},
		{
			name: "gemini configured",
			cfg: func() *Config {
				c := DefaultConfig()
				c.Providers.Gemini.APIKey = "ai-test"
				return c
			}(),
			wantName: "gemini",
		},
		{
			name: "vllm configured",
			cfg: func() *Config {
				c := DefaultConfig()
				c.Providers.VLLM.APIBase = "http://localhost:8000/v1"
				return c
			}(),
			wantName: "vllm",
		},
		{
			name: "copilot takes precedence",
			cfg: func() *Config {
				c := DefaultConfig()
				c.Providers.Copilot.Enabled = true
				c.Providers.Copilot.AccessToken = "token"
				c.Providers.OpenAI.APIKey = "sk-test"
				return c
			}(),
			wantName: "copilot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, _, _ := tt.cfg.GetActiveProvider()
			if name != tt.wantName {
				t.Errorf("GetActiveProvider() name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	// Empty path
	if got := expandPath(""); got != "" {
		t.Errorf("expandPath('') = %q, want empty", got)
	}

	// Tilde expansion
	result := expandPath("~/test")
	if result == "~/test" {
		t.Error("expandPath should expand tilde")
	}
	if result == "" {
		t.Error("expandPath should return non-empty path")
	}

	// Just tilde
	result = expandPath("~")
	if result == "~" {
		t.Error("expandPath('~') should expand to home dir")
	}

	// Absolute path
	result = expandPath("/tmp/test")
	if result != "/tmp/test" {
		t.Errorf("expandPath('/tmp/test') = %q, want /tmp/test", result)
	}
}
