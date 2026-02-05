package cmd

import (
	"fmt"

	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/tui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show configuration status",
	Long:  "Display the current uBot configuration status including provider, channels, and tools.",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Show status using TUI
	return tui.ShowStatus(cfg)
}
