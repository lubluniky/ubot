package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ubot",
	Short: "uBot - Ultra-lightweight personal AI assistant",
	Long:  `uBot is a minimal AI assistant framework with multi-channel support (Telegram, WhatsApp, CLI) and tool execution capabilities.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}
