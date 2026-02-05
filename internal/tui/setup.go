// Package tui provides interactive terminal user interface components for uBot.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/skills"
)

// Provider represents an LLM provider option.
type Provider string

const (
	ProviderOpenRouter Provider = "openrouter"
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderCopilot    Provider = "copilot"
	ProviderOllama     Provider = "ollama"
)

// ModelOptions defines available models for each provider.
var ModelOptions = map[Provider][]string{
	ProviderOpenRouter: {
		"anthropic/claude-opus-4-5",
		"openai/gpt-4o",
		"meta-llama/llama-3.1-70b",
	},
	ProviderAnthropic: {
		"claude-opus-4-5",
		"claude-sonnet-4-20250514",
	},
	ProviderOpenAI: {
		"gpt-4o",
		"gpt-4-turbo",
	},
	ProviderCopilot: {
		"gpt-4o",
	},
	ProviderOllama: {}, // User provides model name
}

// Styles for the setup wizard.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginBottom(1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
)

// SetupState holds the state of the setup wizard.
type SetupState struct {
	Provider       Provider
	APIKey         string
	BaseURL        string
	Model          string
	CustomModel    string
	ConfigTelegram bool
	TelegramToken  string
	TelegramUsers  string
	ConfigWhatsApp bool
	ConfigSearch   bool
	SearchAPIKey   string
	ConfigSkills   bool
	SelectedSkills []string
	Confirmed      bool
}

// RunSetup runs the interactive setup wizard.
// Returns the configured Config or error.
func RunSetup() (*config.Config, error) {
	state := &SetupState{
		BaseURL: "http://localhost:11434",
	}

	// Step 1: Welcome & Provider Selection
	if err := runWelcomeStep(state); err != nil {
		return nil, fmt.Errorf("welcome step failed: %w", err)
	}

	// Step 2: Provider Configuration
	if err := runProviderConfigStep(state); err != nil {
		return nil, fmt.Errorf("provider config step failed: %w", err)
	}

	// Step 3: Model Selection
	if err := runModelSelectionStep(state); err != nil {
		return nil, fmt.Errorf("model selection step failed: %w", err)
	}

	// Step 4: Channels Configuration
	if err := runChannelsStep(state); err != nil {
		return nil, fmt.Errorf("channels step failed: %w", err)
	}

	// Step 5: Web Search Configuration
	if err := runWebSearchStep(state); err != nil {
		return nil, fmt.Errorf("web search step failed: %w", err)
	}

	// Step 6: Skills Configuration
	if err := runSkillsStep(state); err != nil {
		return nil, fmt.Errorf("skills step failed: %w", err)
	}

	// Step 7: Confirmation
	if err := runConfirmationStep(state); err != nil {
		return nil, fmt.Errorf("confirmation step failed: %w", err)
	}

	if !state.Confirmed {
		return nil, fmt.Errorf("setup cancelled by user")
	}

	// Build configuration from state
	cfg := buildConfigFromState(state)

	// Save the configuration
	if err := config.SaveConfig(cfg, ""); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println(successStyle.Render("\n✓ Configuration saved successfully!"))
	fmt.Println(subtitleStyle.Render("Config file: " + config.GetConfigPath()))
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("  Shipped to you by Borkiss"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  https://github.com/lubluniky"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println()

	return cfg, nil
}

// runWelcomeStep displays the welcome message and provider selection.
func runWelcomeStep(state *SetupState) error {
	// ASCII banner
	banner := `
    __  ______        __
   / / / / __ )____  / /_
  / / / / __  / __ \/ __/
 / /_/ / /_/ / /_/ / /_
 \__,_/_____/\____/\__/

 The World's Most Lightweight
    Self-Hosted AI Assistant
`
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(banner))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("  Shipped to you by Borkiss"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("  https://github.com/lubluniky"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println()

	welcome := boxStyle.Render(
		titleStyle.Render("Welcome to uBot Setup") + "\n\n" +
			"This wizard will help you configure uBot.\n" +
			"You can always edit the configuration later at:\n" +
			subtitleStyle.Render(config.GetConfigPath()),
	)
	fmt.Println(welcome)
	fmt.Println()

	var provider string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select your LLM provider").
				Description("Choose the AI provider you want to use").
				Options(
					huh.NewOption("OpenRouter (multiple models, one API)", string(ProviderOpenRouter)),
					huh.NewOption("Anthropic (Claude models)", string(ProviderAnthropic)),
					huh.NewOption("OpenAI (GPT models)", string(ProviderOpenAI)),
					huh.NewOption("GitHub Copilot (free with GitHub)", string(ProviderCopilot)),
					huh.NewOption("Ollama/Local (self-hosted)", string(ProviderOllama)),
				).
				Value(&provider),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	state.Provider = Provider(provider)
	return nil
}

// runProviderConfigStep configures the selected provider.
func runProviderConfigStep(state *SetupState) error {
	switch state.Provider {
	case ProviderOpenRouter, ProviderAnthropic, ProviderOpenAI:
		return runAPIKeyStep(state)
	case ProviderCopilot:
		return runCopilotAuthStep(state)
	case ProviderOllama:
		return runOllamaStep(state)
	default:
		return fmt.Errorf("unknown provider: %s", state.Provider)
	}
}

// runAPIKeyStep prompts for API key.
func runAPIKeyStep(state *SetupState) error {
	var providerName string
	var placeholder string

	switch state.Provider {
	case ProviderOpenRouter:
		providerName = "OpenRouter"
		placeholder = "sk-or-..."
	case ProviderAnthropic:
		providerName = "Anthropic"
		placeholder = "sk-ant-..."
	case ProviderOpenAI:
		providerName = "OpenAI"
		placeholder = "sk-..."
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Enter your %s API key", providerName)).
				Description("Your API key will be stored locally and never shared").
				Placeholder(placeholder).
				EchoMode(huh.EchoModePassword).
				Value(&state.APIKey).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("API key is required")
					}
					return nil
				}),
		),
	)

	return form.Run()
}

// runCopilotAuthStep runs the GitHub Copilot device flow authentication.
func runCopilotAuthStep(state *SetupState) error {
	fmt.Println(subtitleStyle.Render("\nStarting GitHub Copilot authentication..."))

	token, err := RunCopilotAuth()
	if err != nil {
		return fmt.Errorf("copilot authentication failed: %w", err)
	}

	state.APIKey = token
	return nil
}

// runOllamaStep configures Ollama/local provider.
func runOllamaStep(state *SetupState) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Ollama Base URL").
				Description("The URL where your Ollama server is running").
				Placeholder("http://localhost:11434").
				Value(&state.BaseURL),
		),
	)

	return form.Run()
}

// runModelSelectionStep allows user to select or enter a model.
func runModelSelectionStep(state *SetupState) error {
	models := ModelOptions[state.Provider]

	if state.Provider == ProviderOllama || len(models) == 0 {
		// Free-form model input for Ollama
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter model name").
					Description("The name of the model to use (e.g., llama2, mistral, codellama)").
					Placeholder("llama3.2").
					Value(&state.CustomModel).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("model name is required")
						}
						return nil
					}),
			),
		)

		if err := form.Run(); err != nil {
			return err
		}
		state.Model = state.CustomModel
		return nil
	}

	// Select from available models
	options := make([]huh.Option[string], len(models))
	for i, m := range models {
		options[i] = huh.NewOption(m, m)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select model").
				Description("Choose the AI model to use").
				Options(options...).
				Value(&state.Model),
		),
	)

	return form.Run()
}

// runChannelsStep configures communication channels.
func runChannelsStep(state *SetupState) error {
	// Ask about Telegram
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Configure Telegram?").
				Description("Set up a Telegram bot for messaging").
				Value(&state.ConfigTelegram),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if state.ConfigTelegram {
		telegramForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Telegram Bot Token").
					Description("Get this from @BotFather on Telegram").
					Placeholder("123456789:ABCdefGHIjklMNOpqrsTUVwxyz").
					EchoMode(huh.EchoModePassword).
					Value(&state.TelegramToken).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("bot token is required")
						}
						return nil
					}),
				huh.NewInput().
					Title("Allowed User IDs (optional)").
					Description("Comma-separated list of Telegram user IDs that can use the bot").
					Placeholder("123456789, 987654321").
					Value(&state.TelegramUsers),
			),
		)

		if err := telegramForm.Run(); err != nil {
			return err
		}
	}

	// Ask about WhatsApp
	whatsappForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Configure WhatsApp?").
				Description("Requires Node.js for the WhatsApp bridge").
				Value(&state.ConfigWhatsApp),
		),
	)

	if err := whatsappForm.Run(); err != nil {
		return err
	}

	if state.ConfigWhatsApp {
		fmt.Println(warningStyle.Render("\nNote: WhatsApp integration requires Node.js to be installed."))
		fmt.Println(subtitleStyle.Render("The bridge will be configured with default settings."))
	}

	return nil
}

// runWebSearchStep configures web search capability.
func runWebSearchStep(state *SetupState) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable Web Search?").
				Description("Allow the AI to search the web (requires Brave Search API key)").
				Value(&state.ConfigSearch),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if state.ConfigSearch {
		searchForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Brave Search API Key").
					Description("Get your API key from https://brave.com/search/api/").
					Placeholder("BSA...").
					EchoMode(huh.EchoModePassword).
					Value(&state.SearchAPIKey).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("API key is required for web search")
						}
						return nil
					}),
			),
		)

		if err := searchForm.Run(); err != nil {
			return err
		}
	}

	return nil
}

// runSkillsStep configures skills from the remote repository.
func runSkillsStep(state *SetupState) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Install Skills?").
				Description("Download and install AI skills from the community repository").
				Value(&state.ConfigSkills),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if !state.ConfigSkills {
		return nil
	}

	fmt.Println(subtitleStyle.Render("\nFetching available skills..."))

	// Create skills manager
	configDir := config.GetConfigDir()
	workspacePath := config.GetConfigDir() + "/workspace" // Default workspace path
	manager := skills.NewManager(configDir, workspacePath)

	// Clone or update the skills repo
	isNew, err := manager.EnsureRepo()
	if err != nil {
		fmt.Println(warningStyle.Render("Failed to fetch skills repository: " + err.Error()))
		fmt.Println(subtitleStyle.Render("You can install skills manually later."))
		return nil // Don't fail setup
	}

	if isNew {
		fmt.Println(successStyle.Render("Skills repository downloaded!"))
	} else {
		fmt.Println(successStyle.Render("Skills repository updated!"))
	}

	// Discover available skills
	if err := manager.DiscoverAvailable(); err != nil {
		fmt.Println(warningStyle.Render("Failed to discover skills: " + err.Error()))
		return nil
	}

	availableSkills := manager.ListAvailable()
	if len(availableSkills) == 0 {
		fmt.Println(subtitleStyle.Render("No skills found in the repository."))
		return nil
	}

	fmt.Printf(subtitleStyle.Render("Found %d skills available.\n\n"), len(availableSkills))

	// Build options for multi-select
	options := make([]huh.Option[string], 0, len(availableSkills))
	for _, skill := range availableSkills {
		label := skill.Name
		if skill.Category != "" {
			label = fmt.Sprintf("[%s] %s", skill.Category, skill.Name)
		}
		if skill.Title != "" && skill.Title != skill.Name {
			label = fmt.Sprintf("%s - %s", label, skill.Title)
		}
		options = append(options, huh.NewOption(label, skill.Name))
	}

	// Show multi-select form
	skillsForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select skills to install").
				Description("Use space to select, enter to confirm").
				Options(options...).
				Value(&state.SelectedSkills),
		),
	)

	if err := skillsForm.Run(); err != nil {
		return err
	}

	// Install selected skills
	if len(state.SelectedSkills) > 0 {
		fmt.Println(subtitleStyle.Render("\nInstalling selected skills..."))
		results := manager.InstallMultiple(state.SelectedSkills)

		successCount := 0
		for name, err := range results {
			if err != nil {
				fmt.Println(warningStyle.Render(fmt.Sprintf("  Failed to install %s: %s", name, err)))
			} else {
				fmt.Println(successStyle.Render(fmt.Sprintf("  Installed: %s", name)))
				successCount++
			}
		}

		fmt.Printf(subtitleStyle.Render("\nInstalled %d of %d skills.\n"), successCount, len(state.SelectedSkills))
	}

	return nil
}

// runConfirmationStep shows a summary and confirms the configuration.
func runConfirmationStep(state *SetupState) error {
	summary := buildSummary(state)
	fmt.Println(boxStyle.Render(summary))
	fmt.Println()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save this configuration?").
				Affirmative("Yes, save").
				Negative("No, cancel").
				Value(&state.Confirmed),
		),
	)

	return form.Run()
}

// buildSummary creates a text summary of the configuration.
func buildSummary(state *SetupState) string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Configuration Summary"))
	sb.WriteString("\n\n")

	// Provider
	sb.WriteString(fmt.Sprintf("Provider: %s\n", successStyle.Render(string(state.Provider))))
	sb.WriteString(fmt.Sprintf("Model: %s\n", state.Model))

	if state.Provider == ProviderOllama {
		sb.WriteString(fmt.Sprintf("Base URL: %s\n", state.BaseURL))
	}

	sb.WriteString("\n")

	// Channels
	sb.WriteString("Channels:\n")
	if state.ConfigTelegram {
		sb.WriteString(fmt.Sprintf("  Telegram: %s\n", successStyle.Render("enabled")))
	} else {
		sb.WriteString(fmt.Sprintf("  Telegram: %s\n", subtitleStyle.Render("disabled")))
	}

	if state.ConfigWhatsApp {
		sb.WriteString(fmt.Sprintf("  WhatsApp: %s\n", successStyle.Render("enabled")))
	} else {
		sb.WriteString(fmt.Sprintf("  WhatsApp: %s\n", subtitleStyle.Render("disabled")))
	}

	sb.WriteString("\n")

	// Web Search
	if state.ConfigSearch {
		sb.WriteString(fmt.Sprintf("Web Search: %s\n", successStyle.Render("enabled")))
	} else {
		sb.WriteString(fmt.Sprintf("Web Search: %s\n", subtitleStyle.Render("disabled")))
	}

	sb.WriteString("\n")

	// Skills
	if len(state.SelectedSkills) > 0 {
		sb.WriteString(fmt.Sprintf("Skills: %s (%d installed)\n", successStyle.Render("enabled"), len(state.SelectedSkills)))
		for _, skill := range state.SelectedSkills {
			sb.WriteString(fmt.Sprintf("  - %s\n", skill))
		}
	} else {
		sb.WriteString(fmt.Sprintf("Skills: %s\n", subtitleStyle.Render("none")))
	}

	return sb.String()
}

// buildConfigFromState creates a Config from the setup state.
func buildConfigFromState(state *SetupState) *config.Config {
	cfg := config.DefaultConfig()

	// Set model
	model := state.Model
	if model == "" {
		model = state.CustomModel
	}
	cfg.Agents.Defaults.Model = model

	// Configure provider
	switch state.Provider {
	case ProviderOpenRouter:
		cfg.Providers.OpenRouter.APIKey = state.APIKey
	case ProviderAnthropic:
		cfg.Providers.Anthropic.APIKey = state.APIKey
	case ProviderOpenAI:
		cfg.Providers.OpenAI.APIKey = state.APIKey
	case ProviderCopilot:
		cfg.Providers.Copilot.Enabled = true
		cfg.Providers.Copilot.AccessToken = state.APIKey
		cfg.Providers.Copilot.Model = model
	case ProviderOllama:
		cfg.Providers.VLLM.APIBase = state.BaseURL
	}

	// Configure Telegram
	if state.ConfigTelegram {
		cfg.Channels.Telegram.Enabled = true
		cfg.Channels.Telegram.Token = state.TelegramToken
		if state.TelegramUsers != "" {
			users := strings.Split(state.TelegramUsers, ",")
			for i, u := range users {
				users[i] = strings.TrimSpace(u)
			}
			cfg.Channels.Telegram.AllowFrom = users
		}
	}

	// Configure WhatsApp
	if state.ConfigWhatsApp {
		cfg.Channels.WhatsApp.Enabled = true
	}

	// Configure Web Search
	if state.ConfigSearch {
		cfg.Tools.Web.Search.APIKey = state.SearchAPIKey
	}

	return cfg
}
