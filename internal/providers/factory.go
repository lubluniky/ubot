package providers

import (
	"fmt"

	"github.com/hkuds/ubot/internal/config"
)

// Default models for each provider
const (
	DefaultOpenRouterModel = "anthropic/claude-3.5-sonnet"
	DefaultAnthropicModel  = "claude-3-5-sonnet-20241022"
	DefaultOpenAIModel     = "gpt-4o"
	DefaultGeminiModel     = "gemini-1.5-pro"
	DefaultGroqModel       = "llama-3.1-70b-versatile"
	DefaultVLLMModel       = "default"
)

// NewProviderFromConfig creates a Provider based on the configuration.
// It checks providers in priority order: Copilot > MiniMax > OpenRouter > Anthropic > OpenAI > Gemini > Groq > VLLM.
func NewProviderFromConfig(cfg *config.Config) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// Priority 1: Copilot (if enabled)
	if cfg.Providers.Copilot.Enabled && cfg.Providers.Copilot.AccessToken != "" {
		model := cfg.Providers.Copilot.Model
		if model == "" {
			model = CopilotDefaultModel
		}
		return NewCopilotProvider(cfg.Providers.Copilot.AccessToken, model), nil
	}

	// Priority 2: MiniMax (if enabled)
	if cfg.Providers.MiniMax.Enabled && cfg.Providers.MiniMax.APIKey != "" {
		model := cfg.Providers.MiniMax.Model
		if model == "" {
			model = DefaultMiniMaxModel
		}
		return NewMiniMaxProvider(cfg.Providers.MiniMax.APIKey, cfg.Providers.MiniMax.Region, model), nil
	}

	// Priority 3: OpenRouter
	if cfg.Providers.OpenRouter.APIKey != "" {
		apiBase := cfg.Providers.OpenRouter.APIBase
		if apiBase == "" {
			apiBase = "https://openrouter.ai/api/v1"
		}
		return NewOpenAIProvider("openrouter", cfg.Providers.OpenRouter.APIKey, apiBase, DefaultOpenRouterModel), nil
	}

	// Priority 4: Anthropic
	if cfg.Providers.Anthropic.APIKey != "" {
		apiBase := cfg.Providers.Anthropic.APIBase
		if apiBase == "" {
			apiBase = "https://api.anthropic.com/v1"
		}
		return NewOpenAIProvider("anthropic", cfg.Providers.Anthropic.APIKey, apiBase, DefaultAnthropicModel), nil
	}

	// Priority 5: OpenAI
	if cfg.Providers.OpenAI.APIKey != "" {
		apiBase := cfg.Providers.OpenAI.APIBase
		if apiBase == "" {
			apiBase = "https://api.openai.com/v1"
		}
		return NewOpenAIProvider("openai", cfg.Providers.OpenAI.APIKey, apiBase, DefaultOpenAIModel), nil
	}

	// Priority 6: Gemini
	if cfg.Providers.Gemini.APIKey != "" {
		apiBase := cfg.Providers.Gemini.APIBase
		if apiBase == "" {
			apiBase = "https://generativelanguage.googleapis.com/v1beta/openai"
		}
		return NewOpenAIProvider("gemini", cfg.Providers.Gemini.APIKey, apiBase, DefaultGeminiModel), nil
	}

	// Priority 7: Groq
	if cfg.Providers.Groq.APIKey != "" {
		apiBase := cfg.Providers.Groq.APIBase
		if apiBase == "" {
			apiBase = "https://api.groq.com/openai/v1"
		}
		return NewOpenAIProvider("groq", cfg.Providers.Groq.APIKey, apiBase, DefaultGroqModel), nil
	}

	// Priority 8: VLLM (may work without API key for local deployments)
	if cfg.Providers.VLLM.APIBase != "" {
		return NewOpenAIProvider("vllm", cfg.Providers.VLLM.APIKey, cfg.Providers.VLLM.APIBase, DefaultVLLMModel), nil
	}

	return nil, fmt.Errorf("no LLM provider configured: please configure at least one provider in ~/.ubot/config.json")
}

// NewProviderByName creates a specific provider by name.
// This is useful when you want to explicitly use a specific provider regardless of priority.
func NewProviderByName(cfg *config.Config, name string) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	switch name {
	case "copilot":
		if !cfg.Providers.Copilot.Enabled {
			return nil, fmt.Errorf("copilot provider is not enabled")
		}
		if cfg.Providers.Copilot.AccessToken == "" {
			return nil, fmt.Errorf("copilot access token is not configured")
		}
		model := cfg.Providers.Copilot.Model
		if model == "" {
			model = CopilotDefaultModel
		}
		return NewCopilotProvider(cfg.Providers.Copilot.AccessToken, model), nil

	case "minimax":
		if !cfg.Providers.MiniMax.Enabled {
			return nil, fmt.Errorf("minimax provider is not enabled")
		}
		if cfg.Providers.MiniMax.APIKey == "" {
			return nil, fmt.Errorf("minimax API key is not configured")
		}
		model := cfg.Providers.MiniMax.Model
		if model == "" {
			model = DefaultMiniMaxModel
		}
		return NewMiniMaxProvider(cfg.Providers.MiniMax.APIKey, cfg.Providers.MiniMax.Region, model), nil

	case "openrouter":
		if cfg.Providers.OpenRouter.APIKey == "" {
			return nil, fmt.Errorf("openrouter API key is not configured")
		}
		apiBase := cfg.Providers.OpenRouter.APIBase
		if apiBase == "" {
			apiBase = "https://openrouter.ai/api/v1"
		}
		return NewOpenAIProvider("openrouter", cfg.Providers.OpenRouter.APIKey, apiBase, DefaultOpenRouterModel), nil

	case "anthropic":
		if cfg.Providers.Anthropic.APIKey == "" {
			return nil, fmt.Errorf("anthropic API key is not configured")
		}
		apiBase := cfg.Providers.Anthropic.APIBase
		if apiBase == "" {
			apiBase = "https://api.anthropic.com/v1"
		}
		return NewOpenAIProvider("anthropic", cfg.Providers.Anthropic.APIKey, apiBase, DefaultAnthropicModel), nil

	case "openai":
		if cfg.Providers.OpenAI.APIKey == "" {
			return nil, fmt.Errorf("openai API key is not configured")
		}
		apiBase := cfg.Providers.OpenAI.APIBase
		if apiBase == "" {
			apiBase = "https://api.openai.com/v1"
		}
		return NewOpenAIProvider("openai", cfg.Providers.OpenAI.APIKey, apiBase, DefaultOpenAIModel), nil

	case "gemini":
		if cfg.Providers.Gemini.APIKey == "" {
			return nil, fmt.Errorf("gemini API key is not configured")
		}
		apiBase := cfg.Providers.Gemini.APIBase
		if apiBase == "" {
			apiBase = "https://generativelanguage.googleapis.com/v1beta/openai"
		}
		return NewOpenAIProvider("gemini", cfg.Providers.Gemini.APIKey, apiBase, DefaultGeminiModel), nil

	case "groq":
		if cfg.Providers.Groq.APIKey == "" {
			return nil, fmt.Errorf("groq API key is not configured")
		}
		apiBase := cfg.Providers.Groq.APIBase
		if apiBase == "" {
			apiBase = "https://api.groq.com/openai/v1"
		}
		return NewOpenAIProvider("groq", cfg.Providers.Groq.APIKey, apiBase, DefaultGroqModel), nil

	case "vllm":
		if cfg.Providers.VLLM.APIBase == "" {
			return nil, fmt.Errorf("vllm API base URL is not configured")
		}
		return NewOpenAIProvider("vllm", cfg.Providers.VLLM.APIKey, cfg.Providers.VLLM.APIBase, DefaultVLLMModel), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

// ListAvailableProviders returns a list of provider names that are configured
// and can be used.
func ListAvailableProviders(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}

	var providers []string

	if cfg.Providers.Copilot.Enabled && cfg.Providers.Copilot.AccessToken != "" {
		providers = append(providers, "copilot")
	}
	if cfg.Providers.MiniMax.Enabled && cfg.Providers.MiniMax.APIKey != "" {
		providers = append(providers, "minimax")
	}
	if cfg.Providers.OpenRouter.APIKey != "" {
		providers = append(providers, "openrouter")
	}
	if cfg.Providers.Anthropic.APIKey != "" {
		providers = append(providers, "anthropic")
	}
	if cfg.Providers.OpenAI.APIKey != "" {
		providers = append(providers, "openai")
	}
	if cfg.Providers.Gemini.APIKey != "" {
		providers = append(providers, "gemini")
	}
	if cfg.Providers.Groq.APIKey != "" {
		providers = append(providers, "groq")
	}
	if cfg.Providers.VLLM.APIBase != "" {
		providers = append(providers, "vllm")
	}

	return providers
}
