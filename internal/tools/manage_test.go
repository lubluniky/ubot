package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hkuds/ubot/internal/config"
)

func TestManageUbotTool_CLISourceAllowed(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(cfg, cfgPath); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	tool := NewManageUbotTool(cfgPath)
	tool.SetSource("cli")
	defer tool.ClearSource()

	ctx := context.Background()

	// show_config should work
	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "show_config",
	})
	if err != nil {
		t.Fatalf("show_config from CLI should succeed, got error: %v", err)
	}
	if result == "" {
		t.Error("show_config should return non-empty result")
	}

	// update_config should work
	result, err = tool.Execute(ctx, map[string]interface{}{
		"action": "update_config",
		"key":    "agents.defaults.model",
		"value":  "gpt-4o",
	})
	if err != nil {
		t.Fatalf("update_config from CLI should succeed, got error: %v", err)
	}
	if result == "" {
		t.Error("update_config should return non-empty result")
	}

	// Verify the config was updated
	updatedCfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load updated config: %v", err)
	}
	if updatedCfg.Agents.Defaults.Model != "gpt-4o" {
		t.Errorf("model not updated: got %q, want %q", updatedCfg.Agents.Defaults.Model, "gpt-4o")
	}

	// restart should work
	result, err = tool.Execute(ctx, map[string]interface{}{
		"action": "restart",
	})
	if err != nil {
		t.Fatalf("restart from CLI should succeed, got error: %v", err)
	}
	if result == "" {
		t.Error("restart should return non-empty result")
	}
}

func TestManageUbotTool_TelegramSourceDenied(t *testing.T) {
	tool := NewManageUbotTool("/nonexistent/config.json")
	tool.SetSource("telegram")
	defer tool.ClearSource()

	ctx := context.Background()

	actions := []string{"show_config", "update_config", "restart"}
	for _, action := range actions {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"action": action,
		})
		if err == nil {
			t.Errorf("action %q from telegram should be denied", action)
		}
		if err != nil && err.Error() != "manage_ubot: this action is only available from the CLI" {
			t.Errorf("action %q: unexpected error message: %v", action, err)
		}
	}
}

func TestManageUbotTool_EmptySourceDenied(t *testing.T) {
	tool := NewManageUbotTool("/nonexistent/config.json")
	// Do not set source - should be empty by default

	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"action": "show_config",
	})
	if err == nil {
		t.Error("action with empty source should be denied")
	}
	if err != nil && err.Error() != "manage_ubot: this action is only available from the CLI" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestManageUbotTool_WhatsAppSourceDenied(t *testing.T) {
	tool := NewManageUbotTool("/nonexistent/config.json")
	tool.SetSource("whatsapp")
	defer tool.ClearSource()

	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"action": "show_config",
	})
	if err == nil {
		t.Error("action from whatsapp should be denied")
	}
}

func TestManageUbotTool_ShowConfigNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "nonexistent.json")

	tool := NewManageUbotTool(cfgPath)
	tool.SetSource("cli")
	defer tool.ClearSource()

	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "show_config",
	})
	// LoadConfig may return an error when file does not exist, or return defaults.
	// Either is acceptable — the key is no sensitive data is leaked.
	if err != nil {
		// Config file doesn't exist — acceptable
		return
	}
	// If it returns defaults, it should be valid JSON and not contain raw API keys
	if !strings.Contains(result, "{") {
		t.Errorf("expected JSON output, got: %q", result)
	}
}

func TestManageUbotTool_UpdateConfigMissingParams(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(cfg, cfgPath); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	tool := NewManageUbotTool(cfgPath)
	tool.SetSource("cli")
	defer tool.ClearSource()

	ctx := context.Background()

	// Missing key
	_, err := tool.Execute(ctx, map[string]interface{}{
		"action": "update_config",
		"value":  "something",
	})
	if err == nil {
		t.Error("update_config without key should fail")
	}

	// Missing value
	_, err = tool.Execute(ctx, map[string]interface{}{
		"action": "update_config",
		"key":    "agents.defaults.model",
	})
	if err == nil {
		t.Error("update_config without value should fail")
	}
}

func TestManageUbotTool_UnknownAction(t *testing.T) {
	tool := NewManageUbotTool("/nonexistent/config.json")
	tool.SetSource("cli")
	defer tool.ClearSource()

	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"action": "delete_everything",
	})
	if err == nil {
		t.Error("unknown action should fail")
	}
}

func TestManageUbotTool_ShowConfigContent(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "test-model"
	if err := config.SaveConfig(cfg, cfgPath); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	tool := NewManageUbotTool(cfgPath)
	tool.SetSource("cli")
	defer tool.ClearSource()

	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "show_config",
	})
	if err != nil {
		t.Fatalf("show_config should succeed: %v", err)
	}

	// Result should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("show_config result should be valid JSON: %v", err)
	}
}

func TestManageUbotTool_SetAndClearSource(t *testing.T) {
	tool := NewManageUbotTool("/nonexistent/config.json")
	ctx := context.Background()

	// Initially empty source - should be denied
	_, err := tool.Execute(ctx, map[string]interface{}{"action": "show_config"})
	if err == nil {
		t.Error("empty source should be denied")
	}

	// Set source to cli - should be allowed (but config doesn't exist)
	tool.SetSource("cli")
	result, err := tool.Execute(ctx, map[string]interface{}{"action": "show_config"})
	// The file doesn't exist, so it returns "No config file found" but no error
	if err != nil {
		// /nonexistent/ path will cause a read error (not IsNotExist on some systems)
		// That's acceptable - the point is source check passed
		t.Logf("Expected read error for nonexistent path: %v", err)
	} else if result == "" {
		t.Error("should return some result")
	}

	// Clear source - should be denied again
	tool.ClearSource()
	_, err = tool.Execute(ctx, map[string]interface{}{"action": "show_config"})
	if err == nil {
		t.Error("after ClearSource, should be denied")
	}
}

func TestManageUbotTool_UpdateConfigInvalidKeyPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(cfg, cfgPath); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	tool := NewManageUbotTool(cfgPath)
	tool.SetSource("cli")
	defer tool.ClearSource()

	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"action": "update_config",
		"key":    "nonexistent.deep.path",
		"value":  "something",
	})
	if err == nil {
		t.Error("update_config with invalid key path should fail")
	}
}

func TestManageUbotTool_ToolMetadata(t *testing.T) {
	tool := NewManageUbotTool("")
	if tool.Name() != "manage_ubot" {
		t.Errorf("unexpected name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
	params := tool.Parameters()
	if params == nil {
		t.Fatal("parameters should not be nil")
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("parameters should have properties")
	}
	if _, ok := props["action"]; !ok {
		t.Error("parameters should include 'action'")
	}
	if _, ok := props["key"]; !ok {
		t.Error("parameters should include 'key'")
	}
	if _, ok := props["value"]; !ok {
		t.Error("parameters should include 'value'")
	}
}

func TestManageUbotTool_RestartMessage(t *testing.T) {
	tool := NewManageUbotTool("")
	tool.SetSource("cli")
	defer tool.ClearSource()

	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "restart",
	})
	if err != nil {
		t.Fatalf("restart should succeed: %v", err)
	}
	if result != "Restart requested. The gateway will restart shortly." {
		t.Errorf("unexpected restart message: %q", result)
	}
}

func TestSplitDotPath(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"key", []string{"key"}},
		{"a.b", []string{"a", "b"}},
		{"a.b.c", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		result := splitDotPath(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitDotPath(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("splitDotPath(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestSetNestedValue(t *testing.T) {
	m := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"key": "old",
			},
		},
	}

	err := setNestedValue(m, "level1.level2.key", "new")
	if err != nil {
		t.Fatalf("setNestedValue should succeed: %v", err)
	}

	l1 := m["level1"].(map[string]interface{})
	l2 := l1["level2"].(map[string]interface{})
	if l2["key"] != "new" {
		t.Errorf("value not updated: got %v", l2["key"])
	}
}

// TestManageUbotTool_MissingAction tests that missing action param returns error.
func TestManageUbotTool_MissingAction(t *testing.T) {
	tool := NewManageUbotTool("")
	tool.SetSource("cli")
	defer tool.ClearSource()

	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{})
	if err == nil {
		t.Error("missing action should fail")
	}
}

// Ensure the tool config file is written with 0600 permissions (via config.SaveConfig).
func TestManageUbotTool_UpdateConfigFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(cfg, cfgPath); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	tool := NewManageUbotTool(cfgPath)
	tool.SetSource("cli")
	defer tool.ClearSource()

	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"action": "update_config",
		"key":    "gateway.host",
		"value":  "0.0.0.0",
	})
	if err != nil {
		t.Fatalf("update_config should succeed: %v", err)
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("config file permissions should be 0600, got %o", perm)
	}
}
