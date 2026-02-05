package cmd

import (
	"fmt"
	"strings"

	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/skills"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage skills",
	Long:  "List, install, uninstall, and inspect skills for uBot.",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed and available skills",
	Long:  "Show skills installed locally and available from the remote repository.",
	RunE:  runSkillsList,
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Install a skill from the remote repository",
	Long:  "Download and install a skill from the remote skills repository.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsInstall,
}

var skillsUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Remove an installed skill",
	Long:  "Uninstall a skill from the local workspace.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsUninstall,
}

var skillsInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show details of a skill",
	Long:  "Display the title, description, and tools of a skill.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsInfo,
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsCmd.AddCommand(skillsUninstallCmd)
	skillsCmd.AddCommand(skillsInfoCmd)
}

// loadConfigAndPaths loads config and returns configDir and workspacePath.
func loadConfigAndPaths() (string, string, error) {
	cfg, err := config.LoadConfig("")
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}
	configDir := config.GetConfigDir()
	workspacePath := cfg.WorkspacePath()
	return configDir, workspacePath, nil
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	configDir, workspacePath, err := loadConfigAndPaths()
	if err != nil {
		return err
	}

	// List installed skills
	loader := skills.NewLoader(workspacePath)
	if err := loader.Discover(); err != nil {
		return fmt.Errorf("failed to discover installed skills: %w", err)
	}

	installed := loader.List()
	fmt.Println("Installed skills:")
	if len(installed) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, name := range installed {
			s := loader.Get(name)
			if s != nil && s.Description != "" {
				desc := s.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				fmt.Printf("  - %s: %s\n", name, desc)
			} else {
				fmt.Printf("  - %s\n", name)
			}
		}
	}

	// List available skills from remote
	fmt.Println()
	fmt.Println("Available skills (remote):")

	mgr := skills.NewManager(configDir, workspacePath)
	if !mgr.IsCached() {
		fmt.Println("  Fetching skills repository...")
	}
	if _, err := mgr.EnsureRepo(); err != nil {
		fmt.Printf("  (could not fetch remote repository: %v)\n", err)
		return nil
	}
	if err := mgr.DiscoverAvailable(); err != nil {
		fmt.Printf("  (could not discover available skills: %v)\n", err)
		return nil
	}

	available := mgr.ListAvailable()
	if len(available) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, s := range available {
			marker := ""
			if mgr.IsInstalled(s.Name) {
				marker = " [installed]"
			}
			desc := s.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			if s.Category != "" {
				fmt.Printf("  - %s (%s): %s%s\n", s.Name, s.Category, desc, marker)
			} else {
				fmt.Printf("  - %s: %s%s\n", s.Name, desc, marker)
			}
		}
	}

	return nil
}

func runSkillsInstall(cmd *cobra.Command, args []string) error {
	name := args[0]

	configDir, workspacePath, err := loadConfigAndPaths()
	if err != nil {
		return err
	}

	mgr := skills.NewManager(configDir, workspacePath)

	fmt.Printf("Fetching skills repository...\n")
	if _, err := mgr.EnsureRepo(); err != nil {
		return fmt.Errorf("failed to fetch skills repository: %w", err)
	}
	if err := mgr.DiscoverAvailable(); err != nil {
		return fmt.Errorf("failed to discover available skills: %w", err)
	}

	if mgr.GetAvailable(name) == nil {
		return fmt.Errorf("skill %q not found in remote repository", name)
	}

	if err := mgr.Install(name); err != nil {
		return fmt.Errorf("failed to install skill: %w", err)
	}

	fmt.Printf("Skill %q installed successfully.\n", name)
	return nil
}

func runSkillsUninstall(cmd *cobra.Command, args []string) error {
	name := args[0]

	configDir, workspacePath, err := loadConfigAndPaths()
	if err != nil {
		return err
	}

	mgr := skills.NewManager(configDir, workspacePath)

	if !mgr.IsInstalled(name) {
		return fmt.Errorf("skill %q is not installed", name)
	}

	if err := mgr.Uninstall(name); err != nil {
		return fmt.Errorf("failed to uninstall skill: %w", err)
	}

	fmt.Printf("Skill %q uninstalled successfully.\n", name)
	return nil
}

func runSkillsInfo(cmd *cobra.Command, args []string) error {
	name := args[0]

	configDir, workspacePath, err := loadConfigAndPaths()
	if err != nil {
		return err
	}

	// Try installed skill first
	loader := skills.NewLoader(workspacePath)
	if err := loader.Discover(); err != nil {
		return fmt.Errorf("failed to discover installed skills: %w", err)
	}

	if s := loader.Get(name); s != nil {
		printSkillInfo(s, true)
		return nil
	}

	// Try available skill from remote
	mgr := skills.NewManager(configDir, workspacePath)
	if mgr.IsCached() || func() bool { _, err := mgr.EnsureRepo(); return err == nil }() {
		if err := mgr.DiscoverAvailable(); err == nil {
			if a := mgr.GetAvailable(name); a != nil {
				// Parse the full skill file for tools info
				s, err := skills.ParseSkillFile(a.Path)
				if err == nil {
					s.Name = a.Name
					printSkillInfo(s, false)
					return nil
				}
			}
		}
	}

	return fmt.Errorf("skill %q not found", name)
}

func printSkillInfo(s *skills.Skill, installed bool) {
	fmt.Printf("Name:        %s\n", s.Name)
	if s.Title != "" {
		fmt.Printf("Title:       %s\n", s.Title)
	}
	if s.Description != "" {
		fmt.Printf("Description: %s\n", s.Description)
	}
	if len(s.Tools) > 0 {
		fmt.Printf("Tools:       %s\n", strings.Join(s.Tools, ", "))
	}
	if installed {
		fmt.Printf("Status:      installed\n")
		fmt.Printf("Path:        %s\n", s.Path)
	} else {
		fmt.Printf("Status:      available (not installed)\n")
	}
}
