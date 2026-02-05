package cmd

import (
	"fmt"

	"github.com/hkuds/ubot/internal/tui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Run interactive setup wizard",
	Long:  "Run the interactive setup wizard to configure uBot with your preferred LLM provider, channels, and tools.",
	RunE:  runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg, err := tui.RunSetup()
	if err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// Show quick status after setup
	fmt.Println()
	tui.ShowQuickStatus(cfg)

	fmt.Println()
	fmt.Println("You can now:")
	fmt.Println("  - Chat with the agent: ubot agent")
	fmt.Println("  - Start the gateway:   ubot gateway")
	fmt.Println("  - View full status:    ubot status")

	return nil
}
