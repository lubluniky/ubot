package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GitHub OAuth endpoints for Copilot.
const (
	githubDeviceCodeURL  = "https://github.com/login/device/code"
	githubAccessTokenURL = "https://github.com/login/oauth/access_token"
	// Copilot client ID (public, used by VS Code)
	copilotClientID = "Iv1.b507a08c87ecfe98"
)

// DeviceCodeResponse represents the response from the device code request.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// AccessTokenResponse represents the response from the access token request.
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// Styles for the Copilot auth UI.
var (
	codeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(1, 4).
			Align(lipgloss.Center)

	codeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	urlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Underline(true)

	instructionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))
)

// copilotAuthModel is the Bubble Tea model for the Copilot auth flow.
type copilotAuthModel struct {
	deviceCode   *DeviceCodeResponse
	spinner      spinner.Model
	done         bool
	err          error
	accessToken  string
	pollInterval time.Duration
	expiresAt    time.Time
}

// pollMsg is sent when it's time to poll for the access token.
type pollMsg struct{}

// pollResultMsg contains the result of a poll attempt.
type pollResultMsg struct {
	token string
	err   error
	retry bool
}

// newCopilotAuthModel creates a new Copilot auth model.
func newCopilotAuthModel(deviceCode *DeviceCodeResponse) copilotAuthModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return copilotAuthModel{
		deviceCode:   deviceCode,
		spinner:      s,
		pollInterval: time.Duration(deviceCode.Interval) * time.Second,
		expiresAt:    time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second),
	}
}

func (m copilotAuthModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.Tick(m.pollInterval, func(t time.Time) tea.Msg {
			return pollMsg{}
		}),
	)
}

func (m copilotAuthModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.err = fmt.Errorf("authentication cancelled")
			m.done = true
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case pollMsg:
		// Check if expired
		if time.Now().After(m.expiresAt) {
			m.err = fmt.Errorf("authentication timed out")
			m.done = true
			return m, tea.Quit
		}

		// Poll for the token
		return m, func() tea.Msg {
			token, retry, err := pollForAccessToken(m.deviceCode.DeviceCode)
			return pollResultMsg{token: token, err: err, retry: retry}
		}

	case pollResultMsg:
		if msg.err != nil && !msg.retry {
			m.err = msg.err
			m.done = true
			return m, tea.Quit
		}
		if msg.token != "" {
			m.accessToken = msg.token
			m.done = true
			return m, tea.Quit
		}
		// Retry after interval
		return m, tea.Tick(m.pollInterval, func(t time.Time) tea.Msg {
			return pollMsg{}
		})
	}

	return m, nil
}

func (m copilotAuthModel) View() string {
	if m.done {
		if m.err != nil {
			return errorStyle.Render(fmt.Sprintf("\nAuthentication failed: %v\n", m.err))
		}
		return successStyle.Render("\nAuthentication successful!\n")
	}

	var s string

	// Title
	s += titleStyle.Render("GitHub Copilot Authentication")
	s += "\n\n"

	// Instructions
	s += instructionStyle.Render("1. Open this URL in your browser:")
	s += "\n"
	s += "   " + urlStyle.Render(m.deviceCode.VerificationURI)
	s += "\n\n"

	s += instructionStyle.Render("2. Enter this code:")
	s += "\n\n"

	// User code in a prominent box
	codeDisplay := codeStyle.Render(m.deviceCode.UserCode)
	s += codeBoxStyle.Render(codeDisplay)
	s += "\n\n"

	// Spinner
	s += m.spinner.View() + " Waiting for authentication..."
	s += "\n\n"

	// Expiration info
	remaining := time.Until(m.expiresAt)
	s += subtitleStyle.Render(fmt.Sprintf("Code expires in %d minutes", int(remaining.Minutes())))
	s += "\n"

	s += subtitleStyle.Render("Press q or Ctrl+C to cancel")

	return s
}

// RunCopilotAuth runs the device flow authentication for GitHub Copilot.
// Shows the verification URL and code, waits for user to complete.
// Returns the access token or error.
func RunCopilotAuth() (string, error) {
	// Step 1: Request device code
	deviceCode, err := RequestDeviceCode()
	if err != nil {
		return "", fmt.Errorf("failed to request device code: %w", err)
	}

	// Step 2: Display UI and poll for token
	model := newCopilotAuthModel(deviceCode)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("TUI error: %w", err)
	}

	result := finalModel.(copilotAuthModel)
	if result.err != nil {
		return "", result.err
	}

	return result.accessToken, nil
}

// RequestDeviceCode requests a new device code from GitHub.
func RequestDeviceCode() (*DeviceCodeResponse, error) {
	payload := map[string]string{
		"client_id": copilotClientID,
		"scope":     "read:user",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", githubDeviceCodeURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get device code: %s", string(body))
	}

	var deviceCode DeviceCodeResponse
	if err := json.Unmarshal(body, &deviceCode); err != nil {
		return nil, err
	}

	return &deviceCode, nil
}

// pollForAccessToken polls GitHub for the access token.
// Returns (token, shouldRetry, error).
func pollForAccessToken(deviceCode string) (string, bool, error) {
	payload := map[string]string{
		"client_id":   copilotClientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", false, err
	}

	req, err := http.NewRequest("POST", githubAccessTokenURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", true, err // Network error, retry
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true, err
	}

	var tokenResp AccessTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", false, fmt.Errorf("failed to parse response: %s", string(body))
	}

	// Handle pending/slow_down errors - these are expected during polling
	switch tokenResp.Error {
	case "authorization_pending":
		return "", true, nil
	case "slow_down":
		// GitHub wants us to slow down
		time.Sleep(5 * time.Second)
		return "", true, nil
	case "expired_token":
		return "", false, fmt.Errorf("device code expired")
	case "access_denied":
		return "", false, fmt.Errorf("access denied by user")
	case "":
		// Success!
		if tokenResp.AccessToken != "" {
			return tokenResp.AccessToken, false, nil
		}
		return "", false, fmt.Errorf("no access token in response")
	default:
		return "", false, fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
}
