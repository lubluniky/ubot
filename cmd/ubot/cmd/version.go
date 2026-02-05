package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version is the current version of uBot, set at build time.
	Version = "0.1.0"
	// GitCommit is the git commit hash, set at build time.
	GitCommit = "dev"
	// BuildDate is the build date, set at build time.
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  "Display the version, build information, and runtime details of uBot.",
	Run:   runVersion,
}

func runVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("uBot %s\n", Version)
	fmt.Printf("  Git commit: %s\n", GitCommit)
	fmt.Printf("  Build date: %s\n", BuildDate)
	fmt.Printf("  Go version: %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
