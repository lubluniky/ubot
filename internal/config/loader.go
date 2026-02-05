package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultConfigDir is the default config directory name.
	DefaultConfigDir = ".ubot"
	// DefaultConfigFile is the default config file name.
	DefaultConfigFile = "config.json"
)

// GetConfigDir returns the default config directory path (~/.ubot).
func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", DefaultConfigDir)
	}
	return filepath.Join(home, DefaultConfigDir)
}

// GetConfigPath returns the default config file path (~/.ubot/config.json).
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), DefaultConfigFile)
}

// LoadConfig loads configuration from the specified path.
// If path is empty, it uses the default config path (~/.ubot/config.json).
// If the config file doesn't exist, it returns the default configuration.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = GetConfigPath()
	}

	// Expand ~ in the path
	path = expandPath(path)

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return DefaultConfig(), nil
	}

	// Read the config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Start with defaults and unmarshal over them
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return cfg, nil
}

// SaveConfig saves the configuration to the specified path.
// If path is empty, it uses the default config path (~/.ubot/config.json).
func SaveConfig(cfg *Config, path string) error {
	if path == "" {
		path = GetConfigPath()
	}

	// Expand ~ in the path
	path = expandPath(path)

	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	// Marshal config to JSON with indentation for readability
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write the config file with secure permissions (readable/writable by owner only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}

	return nil
}

// EnsureConfigDir ensures the config directory (~/.ubot) exists.
// Creates the directory with appropriate permissions if it doesn't exist.
func EnsureConfigDir() error {
	dir := GetConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}
	return nil
}

// Exists checks if a config file exists at the given path.
// If path is empty, checks the default config path.
func Exists(path string) bool {
	if path == "" {
		path = GetConfigPath()
	}
	path = expandPath(path)
	_, err := os.Stat(path)
	return err == nil
}

// InitConfig creates a default config file if it doesn't exist.
// Returns nil if the config already exists or was created successfully.
func InitConfig() error {
	path := GetConfigPath()

	if Exists(path) {
		return nil
	}

	cfg := DefaultConfig()
	return SaveConfig(cfg, path)
}

// EnsureWorkspaceDir ensures the workspace directory exists.
func EnsureWorkspaceDir(cfg *Config) error {
	workspace := cfg.WorkspacePath()
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory %s: %w", workspace, err)
	}
	return nil
}
