package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hkuds/ubot/internal/config"
)

// Status display styles.
var (
	statusTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				MarginBottom(1).
				Padding(0, 1)

	statusBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(60)

	statusSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")).
				MarginTop(1)

	statusLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Width(20)

	statusValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255"))

	statusEnabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	statusDisabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	statusWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	statusDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// ShowStatus displays the current configuration status.
func ShowStatus(cfg *config.Config) error {
	var sb strings.Builder

	// Title
	title := statusTitleStyle.Render("uBot Configuration Status")
	sb.WriteString(title)
	sb.WriteString("\n\n")

	// Provider section
	sb.WriteString(statusSectionStyle.Render("Provider"))
	sb.WriteString("\n")
	sb.WriteString(renderProviderStatus(cfg))
	sb.WriteString("\n")

	// Channels section
	sb.WriteString(statusSectionStyle.Render("Channels"))
	sb.WriteString("\n")
	sb.WriteString(renderChannelsStatus(cfg))
	sb.WriteString("\n")

	// Tools section
	sb.WriteString(statusSectionStyle.Render("Tools"))
	sb.WriteString("\n")
	sb.WriteString(renderToolsStatus(cfg))
	sb.WriteString("\n")

	// Workspace section
	sb.WriteString(statusSectionStyle.Render("Workspace"))
	sb.WriteString("\n")
	sb.WriteString(renderWorkspaceStatus(cfg))

	// Render in a box
	content := statusBoxStyle.Render(sb.String())
	fmt.Println(content)

	return nil
}

// renderProviderStatus renders the provider configuration status.
func renderProviderStatus(cfg *config.Config) string {
	var sb strings.Builder

	providerName, apiKey, apiBase := cfg.GetActiveProvider()

	if providerName == "" {
		sb.WriteString(renderStatusRow("Status", statusErrorStyle.Render("No provider configured")))
		sb.WriteString(renderStatusRow("", statusWarningStyle.Render("Run 'ubot setup' to configure")))
		return sb.String()
	}

	// Provider name
	sb.WriteString(renderStatusRow("Active", statusEnabledStyle.Render(strings.ToUpper(providerName))))

	// Model
	model := cfg.Agents.Defaults.Model
	if model != "" {
		sb.WriteString(renderStatusRow("Model", statusValueStyle.Render(model)))
	}

	// API Base (if custom)
	if apiBase != "" && !isDefaultAPIBase(providerName, apiBase) {
		sb.WriteString(renderStatusRow("API Base", statusValueStyle.Render(apiBase)))
	}

	// API Key status (masked)
	if apiKey != "" {
		masked := maskAPIKey(apiKey)
		sb.WriteString(renderStatusRow("API Key", statusValueStyle.Render(masked)))
	}

	return sb.String()
}

// renderChannelsStatus renders the channels configuration status.
func renderChannelsStatus(cfg *config.Config) string {
	var sb strings.Builder

	// Telegram
	if cfg.Channels.Telegram.Enabled {
		sb.WriteString(renderStatusRow("Telegram", statusEnabledStyle.Render("enabled")))
		if len(cfg.Channels.Telegram.AllowFrom) > 0 {
			users := strings.Join(cfg.Channels.Telegram.AllowFrom, ", ")
			if len(users) > 30 {
				users = users[:27] + "..."
			}
			sb.WriteString(renderStatusRow("  Allowed", statusValueStyle.Render(users)))
		} else {
			sb.WriteString(renderStatusRow("  Allowed", statusWarningStyle.Render("all users (not recommended)")))
		}
	} else {
		sb.WriteString(renderStatusRow("Telegram", statusDisabledStyle.Render("disabled")))
	}

	// WhatsApp
	if cfg.Channels.WhatsApp.Enabled {
		sb.WriteString(renderStatusRow("WhatsApp", statusEnabledStyle.Render("enabled")))
		sb.WriteString(renderStatusRow("  Bridge", statusValueStyle.Render(cfg.Channels.WhatsApp.BridgeURL)))
	} else {
		sb.WriteString(renderStatusRow("WhatsApp", statusDisabledStyle.Render("disabled")))
	}

	return sb.String()
}

// renderToolsStatus renders the tools configuration status.
func renderToolsStatus(cfg *config.Config) string {
	var sb strings.Builder

	// Web Search
	if cfg.Tools.Web.Search.APIKey != "" {
		sb.WriteString(renderStatusRow("Web Search", statusEnabledStyle.Render("enabled")))
		sb.WriteString(renderStatusRow("  Max Results", statusValueStyle.Render(fmt.Sprintf("%d", cfg.Tools.Web.Search.MaxResults))))
	} else {
		sb.WriteString(renderStatusRow("Web Search", statusDisabledStyle.Render("disabled")))
	}

	// Shell Execution
	sb.WriteString(renderStatusRow("Shell Exec", statusEnabledStyle.Render("enabled")))
	sb.WriteString(renderStatusRow("  Timeout", statusValueStyle.Render(fmt.Sprintf("%ds", cfg.Tools.Exec.Timeout))))
	if cfg.Tools.Exec.RestrictToWorkspace {
		sb.WriteString(renderStatusRow("  Restricted", statusValueStyle.Render("workspace only")))
	} else {
		sb.WriteString(renderStatusRow("  Restricted", statusWarningStyle.Render("no restrictions")))
	}

	return sb.String()
}

// renderWorkspaceStatus renders the workspace configuration status.
func renderWorkspaceStatus(cfg *config.Config) string {
	var sb strings.Builder

	workspace := cfg.WorkspacePath()
	sb.WriteString(renderStatusRow("Path", statusValueStyle.Render(workspace)))

	// Agent defaults
	sb.WriteString(renderStatusRow("Max Tokens", statusValueStyle.Render(fmt.Sprintf("%d", cfg.Agents.Defaults.MaxTokens))))
	sb.WriteString(renderStatusRow("Temperature", statusValueStyle.Render(fmt.Sprintf("%.1f", cfg.Agents.Defaults.Temperature))))
	sb.WriteString(renderStatusRow("Max Iterations", statusValueStyle.Render(fmt.Sprintf("%d", cfg.Agents.Defaults.MaxToolIterations))))

	return sb.String()
}

// renderStatusRow renders a label-value row.
func renderStatusRow(label, value string) string {
	if label == "" {
		return fmt.Sprintf("  %s\n", value)
	}
	return fmt.Sprintf("  %s %s\n",
		statusLabelStyle.Render(label+":"),
		value,
	)
}

// maskAPIKey masks an API key for display.
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// isDefaultAPIBase checks if the API base is the default for the provider.
func isDefaultAPIBase(provider, apiBase string) bool {
	defaults := map[string]string{
		"openrouter": "https://openrouter.ai/api/v1",
		"anthropic":  "https://api.anthropic.com/v1",
		"openai":     "https://api.openai.com/v1",
		"groq":       "https://api.groq.com/openai/v1",
		"gemini":     "https://generativelanguage.googleapis.com/v1beta",
	}

	if defaultBase, ok := defaults[provider]; ok {
		return apiBase == defaultBase
	}
	return false
}

// ShowQuickStatus shows a minimal one-line status.
func ShowQuickStatus(cfg *config.Config) {
	providerName, _, _ := cfg.GetActiveProvider()

	var status string
	if providerName == "" {
		status = statusErrorStyle.Render("Not configured")
	} else {
		model := cfg.Agents.Defaults.Model
		status = fmt.Sprintf("%s using %s",
			statusEnabledStyle.Render(strings.ToUpper(providerName)),
			statusValueStyle.Render(model),
		)
	}

	// Count enabled channels
	channels := 0
	if cfg.Channels.Telegram.Enabled {
		channels++
	}
	if cfg.Channels.WhatsApp.Enabled {
		channels++
	}

	channelStatus := statusDisabledStyle.Render("no channels")
	if channels > 0 {
		channelStatus = statusEnabledStyle.Render(fmt.Sprintf("%d channel(s)", channels))
	}

	fmt.Printf("uBot: %s | %s\n", status, channelStatus)
}

// ShowProviderList shows a list of all providers and their status.
func ShowProviderList(cfg *config.Config) {
	title := statusTitleStyle.Render("Provider Status")
	fmt.Println(title)
	fmt.Println()

	providers := []struct {
		name      string
		configured bool
		apiKey    string
	}{
		{"OpenRouter", cfg.Providers.OpenRouter.APIKey != "", cfg.Providers.OpenRouter.APIKey},
		{"Anthropic", cfg.Providers.Anthropic.APIKey != "", cfg.Providers.Anthropic.APIKey},
		{"OpenAI", cfg.Providers.OpenAI.APIKey != "", cfg.Providers.OpenAI.APIKey},
		{"Groq", cfg.Providers.Groq.APIKey != "", cfg.Providers.Groq.APIKey},
		{"Gemini", cfg.Providers.Gemini.APIKey != "", cfg.Providers.Gemini.APIKey},
		{"Copilot", cfg.Providers.Copilot.Enabled && cfg.Providers.Copilot.AccessToken != "", cfg.Providers.Copilot.AccessToken},
		{"VLLM/Local", cfg.Providers.VLLM.APIBase != "", ""},
	}

	activeProvider, _, _ := cfg.GetActiveProvider()

	for _, p := range providers {
		var status string
		if p.configured {
			if strings.ToLower(p.name) == activeProvider ||
			   (p.name == "VLLM/Local" && activeProvider == "vllm") ||
			   (p.name == "Copilot" && activeProvider == "copilot") {
				status = statusEnabledStyle.Render("[ACTIVE]")
			} else {
				status = statusValueStyle.Render("[configured]")
			}
		} else {
			status = statusDisabledStyle.Render("[not configured]")
		}

		fmt.Printf("  %-12s %s\n", p.name, status)
	}
	fmt.Println()
}
