package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/hkuds/ubot/internal/config"
)

// ManageUbotTool provides self-management capabilities for ubot.
// Actions are restricted to CLI-only access for security.
type ManageUbotTool struct {
	BaseTool
	source     string
	configPath string
	mu         sync.RWMutex
}

// NewManageUbotTool creates a new ManageUbotTool with the given config path.
// If configPath is empty, the default config path is used.
func NewManageUbotTool(configPath string) *ManageUbotTool {
	if configPath == "" {
		configPath = config.GetConfigPath()
	}

	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "The management action to perform",
				"enum":        []string{"restart", "update_config", "show_config"},
			},
			"key": map[string]interface{}{
				"type":        "string",
				"description": "The config key to update (for update_config action, dot-separated path e.g. 'agents.defaults.model')",
			},
			"value": map[string]interface{}{
				"type":        "string",
				"description": "The new value to set (for update_config action)",
			},
		},
		"required": []string{"action"},
	}

	return &ManageUbotTool{
		BaseTool: NewBaseTool(
			"manage_ubot",
			"Manage ubot configuration and lifecycle. Actions: show_config (display current config), update_config (change a config value), restart (request a restart). Only available from CLI.",
			parameters,
		),
		configPath: configPath,
	}
}

// SetSource sets the current channel source for access control.
func (t *ManageUbotTool) SetSource(source string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.source = source
}

// ClearSource clears the current source after request processing.
func (t *ManageUbotTool) ClearSource() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.source = ""
}

// Execute runs the management action.
func (t *ManageUbotTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	// Check source access
	t.mu.RLock()
	source := t.source
	t.mu.RUnlock()

	if source != "cli" {
		return "", errors.New("manage_ubot: this action is only available from the CLI")
	}

	// Extract action (required)
	action, err := GetStringParam(params, "action")
	if err != nil {
		return "", fmt.Errorf("manage_ubot: %w", err)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("manage_ubot: cancelled: %w", ctx.Err())
	default:
	}

	switch action {
	case "show_config":
		return t.showConfig()
	case "update_config":
		return t.updateConfig(params)
	case "restart":
		return t.restart()
	default:
		return "", fmt.Errorf("manage_ubot: unknown action %q, expected one of: restart, update_config, show_config", action)
	}
}

// showConfig reads and returns the current configuration.
func (t *ManageUbotTool) showConfig() (string, error) {
	data, err := os.ReadFile(t.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "No config file found. Using default configuration.", nil
		}
		return "", fmt.Errorf("manage_ubot: failed to read config: %w", err)
	}
	return string(data), nil
}

// updateConfig updates a config key with a new value.
func (t *ManageUbotTool) updateConfig(params map[string]interface{}) (string, error) {
	key, err := GetStringParam(params, "key")
	if err != nil {
		return "", fmt.Errorf("manage_ubot: update_config requires 'key' parameter: %w", err)
	}

	value, err := GetStringParam(params, "value")
	if err != nil {
		return "", fmt.Errorf("manage_ubot: update_config requires 'value' parameter: %w", err)
	}

	if key == "" {
		return "", errors.New("manage_ubot: key cannot be empty")
	}

	// Load current config
	cfg, err := config.LoadConfig(t.configPath)
	if err != nil {
		return "", fmt.Errorf("manage_ubot: failed to load config: %w", err)
	}

	// Convert config to a generic map for dynamic key update
	cfgData, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("manage_ubot: failed to marshal config: %w", err)
	}

	var cfgMap map[string]interface{}
	if err := json.Unmarshal(cfgData, &cfgMap); err != nil {
		return "", fmt.Errorf("manage_ubot: failed to unmarshal config: %w", err)
	}

	// Set the value using dot-separated key path
	if err := setNestedValue(cfgMap, key, value); err != nil {
		return "", fmt.Errorf("manage_ubot: %w", err)
	}

	// Convert back to Config struct
	updatedData, err := json.Marshal(cfgMap)
	if err != nil {
		return "", fmt.Errorf("manage_ubot: failed to marshal updated config: %w", err)
	}

	var updatedCfg config.Config
	if err := json.Unmarshal(updatedData, &updatedCfg); err != nil {
		return "", fmt.Errorf("manage_ubot: failed to parse updated config: %w", err)
	}

	// Save the updated config
	if err := config.SaveConfig(&updatedCfg, t.configPath); err != nil {
		return "", fmt.Errorf("manage_ubot: failed to save config: %w", err)
	}

	return fmt.Sprintf("Config updated: %s = %s", key, value), nil
}

// restart returns a message indicating restart was requested.
func (t *ManageUbotTool) restart() (string, error) {
	return "Restart requested. The gateway will restart shortly.", nil
}

// setNestedValue sets a value in a nested map using a dot-separated key path.
func setNestedValue(m map[string]interface{}, key, value string) error {
	parts := splitDotPath(key)
	if len(parts) == 0 {
		return errors.New("invalid key path")
	}

	// Navigate to the parent map
	current := m
	for i := 0; i < len(parts)-1; i++ {
		next, ok := current[parts[i]]
		if !ok {
			return fmt.Errorf("key path %q not found at %q", key, parts[i])
		}
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("key path %q: %q is not an object", key, parts[i])
		}
		current = nextMap
	}

	// Set the final value
	lastKey := parts[len(parts)-1]
	current[lastKey] = value
	return nil
}

// splitDotPath splits a dot-separated path into parts.
func splitDotPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}
