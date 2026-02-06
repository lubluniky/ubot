// Package skills provides skill management functionality for uBot.
// This file implements the Manager for cloning, caching, and installing skills
// from remote repositories.
package skills

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	// DefaultSkillsRepo is the default repository URL for skills.
	DefaultSkillsRepo = "https://github.com/anthropics/knowledge-work-plugins"
	// DefaultCacheDir is the default cache directory name.
	DefaultCacheDir = "cache/skills-repo"
)

// AvailableSkill represents a skill available in the source repository.
type AvailableSkill struct {
	Name        string // Directory name (skill identifier)
	Title       string // From # heading
	Description string // First paragraph
	Category    string // Parent directory (e.g., "product-management")
	Path        string // Full path to SKILL.md in cache
}

// Manager handles skill repository cloning and installation.
type Manager struct {
	repoURL       string
	cacheDir      string // ~/.ubot/cache/skills-repo
	workspaceDir  string // ~/.ubot/workspace/skills
	available     map[string]*AvailableSkill
	mu            sync.RWMutex
}

// NewManager creates a new skill manager.
// configDir is typically ~/.ubot
// workspacePath is typically ~/.ubot/workspace
func NewManager(configDir, workspacePath string) *Manager {
	return &Manager{
		repoURL:      DefaultSkillsRepo,
		cacheDir:     filepath.Join(configDir, DefaultCacheDir),
		workspaceDir: filepath.Join(workspacePath, "skills"),
		available:    make(map[string]*AvailableSkill),
	}
}

// SetRepoURL sets a custom repository URL.
func (m *Manager) SetRepoURL(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repoURL = url
}

// GetCacheDir returns the cache directory path.
func (m *Manager) GetCacheDir() string {
	return m.cacheDir
}

// GetWorkspaceDir returns the workspace skills directory path.
func (m *Manager) GetWorkspaceDir() string {
	return m.workspaceDir
}

// IsCached checks if the skills repo is already cached.
func (m *Manager) IsCached() bool {
	gitDir := filepath.Join(m.cacheDir, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

// EnsureRepo clones or updates the skills repository.
// Returns true if repo was freshly cloned, false if updated.
func (m *Manager) EnsureRepo() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already cloned
	if m.IsCached() {
		// Pull latest changes
		if err := m.pullRepo(); err != nil {
			// Pull failed, but repo exists - continue anyway
			return false, nil
		}
		return false, nil
	}

	// Clone the repository
	if err := m.cloneRepo(); err != nil {
		return false, fmt.Errorf("failed to clone skills repo: %w", err)
	}

	return true, nil
}

// cloneRepo clones the repository to the cache directory.
func (m *Manager) cloneRepo() error {
	// Ensure parent directory exists
	parentDir := filepath.Dir(m.cacheDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache parent directory: %w", err)
	}

	// Clone with depth 1 for faster download
	cmd := exec.Command("git", "clone", "--depth", "1", m.repoURL, m.cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	return nil
}

// pullRepo pulls the latest changes from the remote.
func (m *Manager) pullRepo() error {
	cmd := exec.Command("git", "-C", m.cacheDir, "pull", "--ff-only")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	return nil
}

// DiscoverAvailable scans the cached repo for available skills.
// Must call EnsureRepo() first.
func (m *Manager) DiscoverAvailable() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing
	m.available = make(map[string]*AvailableSkill)

	// Check if cache exists
	if !m.IsCached() {
		return fmt.Errorf("skills repo not cached, call EnsureRepo() first")
	}

	// Walk the cache directory looking for SKILL.md files
	// Structure: cache/skills-repo/<category>/<skill-name>/SKILL.md
	// or: cache/skills-repo/<skill-name>/SKILL.md
	err := filepath.Walk(m.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Look for SKILL.md files
		if info.Name() != "SKILL.md" {
			return nil
		}

		// Parse the skill
		skill, err := ParseSkillFile(path)
		if err != nil {
			return nil // Skip unparseable skills
		}

		// Determine skill name and category from path
		relPath, _ := filepath.Rel(m.cacheDir, path)
		parts := strings.Split(filepath.Dir(relPath), string(filepath.Separator))

		var skillName, category string
		if len(parts) >= 2 {
			// Has category: category/skill-name/SKILL.md
			category = parts[0]
			skillName = parts[1]
		} else if len(parts) == 1 {
			// No category: skill-name/SKILL.md
			skillName = parts[0]
		} else {
			return nil // Invalid structure
		}

		// Skip if skill name is empty or looks like metadata
		if skillName == "" || skillName == "." || strings.HasPrefix(skillName, ".") {
			return nil
		}

		m.available[skillName] = &AvailableSkill{
			Name:        skillName,
			Title:       skill.Title,
			Description: skill.Description,
			Category:    category,
			Path:        path,
		}

		return nil
	})

	return err
}

// DiscoverBundled scans a local bundled skills directory and adds them
// to the available skills map. Skills already discovered from the remote
// repo are NOT overwritten (remote takes precedence for same-named skills).
func (m *Manager) DiscoverBundled(bundledPath string) error {
	if bundledPath == "" {
		return nil
	}

	info, err := os.Stat(bundledPath)
	if err != nil || !info.IsDir() {
		return nil // path doesn't exist or isn't a directory, skip silently
	}

	entries, err := os.ReadDir(bundledPath)
	if err != nil {
		return fmt.Errorf("failed to read bundled skills dir: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()

		// Don't overwrite remote skills with same name
		if _, exists := m.available[skillName]; exists {
			continue
		}

		skillFile := filepath.Join(bundledPath, skillName, "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			continue
		}

		skill, err := ParseSkillFile(skillFile)
		if err != nil {
			continue
		}

		m.available[skillName] = &AvailableSkill{
			Name:        skillName,
			Title:       skill.Title,
			Description: skill.Description,
			Category:    "bundled",
			Path:        skillFile,
		}
	}

	return nil
}

// ListAvailable returns a sorted list of available skills.
func (m *Manager) ListAvailable() []*AvailableSkill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skills := make([]*AvailableSkill, 0, len(m.available))
	for _, s := range m.available {
		skills = append(skills, s)
	}

	// Sort by category, then name
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Category != skills[j].Category {
			return skills[i].Category < skills[j].Category
		}
		return skills[i].Name < skills[j].Name
	})

	return skills
}

// ListAvailableNames returns a sorted list of available skill names.
func (m *Manager) ListAvailableNames() []string {
	skills := m.ListAvailable()
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	return names
}

// GetAvailable returns an available skill by name.
func (m *Manager) GetAvailable(name string) *AvailableSkill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.available[name]
}

// Install copies a skill from the cache to the workspace.
func (m *Manager) Install(skillName string) error {
	m.mu.RLock()
	skill, exists := m.available[skillName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("skill %q not found in available skills", skillName)
	}

	// Source directory (containing SKILL.md)
	srcDir := filepath.Dir(skill.Path)

	// Destination directory
	dstDir := filepath.Join(m.workspaceDir, skillName)

	// Ensure workspace skills directory exists
	if err := os.MkdirAll(m.workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace skills directory: %w", err)
	}

	// Remove existing if present
	if err := os.RemoveAll(dstDir); err != nil {
		return fmt.Errorf("failed to remove existing skill: %w", err)
	}

	// Copy the skill directory
	if err := copyDir(srcDir, dstDir); err != nil {
		return fmt.Errorf("failed to copy skill: %w", err)
	}

	return nil
}

// InstallMultiple installs multiple skills.
// Returns a map of skill names to errors (nil for success).
func (m *Manager) InstallMultiple(skillNames []string) map[string]error {
	results := make(map[string]error)
	for _, name := range skillNames {
		results[name] = m.Install(name)
	}
	return results
}

// Uninstall removes a skill from the workspace.
func (m *Manager) Uninstall(skillName string) error {
	skillDir := filepath.Join(m.workspaceDir, skillName)

	// Check if skill exists
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return fmt.Errorf("skill %q not installed", skillName)
	}

	// Remove the skill directory
	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("failed to remove skill: %w", err)
	}

	return nil
}

// ListInstalled returns a list of installed skill names.
func (m *Manager) ListInstalled() ([]string, error) {
	entries, err := os.ReadDir(m.workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read workspace skills directory: %w", err)
	}

	var installed []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if SKILL.md exists
		skillFile := filepath.Join(m.workspaceDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			installed = append(installed, entry.Name())
		}
	}

	sort.Strings(installed)
	return installed, nil
}

// IsInstalled checks if a skill is installed.
func (m *Manager) IsInstalled(skillName string) bool {
	skillFile := filepath.Join(m.workspaceDir, skillName, "SKILL.md")
	_, err := os.Stat(skillFile)
	return err == nil
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	// Get source info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	// Open source
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get source info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Create destination
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}
