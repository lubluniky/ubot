// Package skills provides skill discovery, loading, and management for uBot.
// Skills are markdown files (SKILL.md) that extend the agent's capabilities
// with domain-specific knowledge and tool descriptions.
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Skill represents a loaded skill from SKILL.md
type Skill struct {
	Name        string   // Directory name
	Title       string   // From # heading
	Description string   // First paragraph
	Tools       []string // Tool names mentioned
	Content     string   // Full markdown content
	Path        string   // Path to SKILL.md
	AlwaysLoad  bool     // Load in every context
}

// Loader manages skill discovery and loading
type Loader struct {
	workspacePath string
	bundledPath   string // For bundled skills in binary
	skills        map[string]*Skill
	mu            sync.RWMutex
}

// NewLoader creates a new skill loader with the given workspace path.
// workspacePath is typically ~/.ubot/workspace
func NewLoader(workspacePath string) *Loader {
	return &Loader{
		workspacePath: workspacePath,
		bundledPath:   "", // Can be set later for embedded skills
		skills:        make(map[string]*Skill),
	}
}

// SetBundledPath sets the path to bundled skills (for embedded binary skills).
func (l *Loader) SetBundledPath(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.bundledPath = path
}

// Discover finds all SKILL.md files in the workspace and bundled paths.
// It populates the internal skills map with discovered skills.
func (l *Loader) Discover() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Clear existing skills
	l.skills = make(map[string]*Skill)

	// Search paths in order of priority (user skills override bundled)
	searchPaths := []string{}

	// Add bundled path first (lower priority)
	if l.bundledPath != "" {
		searchPaths = append(searchPaths, l.bundledPath)
	}

	// Add workspace skills path (higher priority)
	if l.workspacePath != "" {
		userSkillsPath := filepath.Join(l.workspacePath, "skills")
		searchPaths = append(searchPaths, userSkillsPath)
	}

	for _, basePath := range searchPaths {
		if err := l.discoverInPath(basePath); err != nil {
			// Log but don't fail - path might not exist
			continue
		}
	}

	return nil
}

// discoverInPath finds SKILL.md files in a given base path.
func (l *Loader) discoverInPath(basePath string) error {
	// Check if path exists
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Path doesn't exist, that's okay
		}
		return fmt.Errorf("cannot access path %s: %w", basePath, err)
	}

	if !info.IsDir() {
		return nil
	}

	// List directories in the base path
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", basePath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillDir := filepath.Join(basePath, skillName)
		skillFile := filepath.Join(skillDir, "SKILL.md")

		// Check if SKILL.md exists
		if _, err := os.Stat(skillFile); err != nil {
			continue // No SKILL.md in this directory
		}

		// Parse the skill file
		skill, err := ParseSkillFile(skillFile)
		if err != nil {
			// Log error but continue with other skills
			continue
		}

		skill.Name = skillName
		skill.Path = skillFile

		// Store skill (later paths override earlier ones)
		l.skills[skillName] = skill
	}

	return nil
}

// Load loads a specific skill by name.
// Returns an error if the skill is not found.
func (l *Loader) Load(name string) (*Skill, error) {
	l.mu.RLock()
	skill, exists := l.skills[name]
	l.mu.RUnlock()

	if exists {
		return skill, nil
	}

	// Try to discover and load
	if err := l.Discover(); err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}

	l.mu.RLock()
	skill, exists = l.skills[name]
	l.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("skill %q not found", name)
	}

	return skill, nil
}

// GetAlwaysLoad returns skills marked as always load.
func (l *Loader) GetAlwaysLoad() []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var alwaysLoad []*Skill
	for _, skill := range l.skills {
		if skill.AlwaysLoad {
			alwaysLoad = append(alwaysLoad, skill)
		}
	}

	// Sort by name for consistent ordering
	sort.Slice(alwaysLoad, func(i, j int) bool {
		return alwaysLoad[i].Name < alwaysLoad[j].Name
	})

	return alwaysLoad
}

// List returns a sorted list of available skill names.
func (l *Loader) List() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.skills))
	for name := range l.skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetSummary returns a brief summary of available skills for the system prompt.
func (l *Loader) GetSummary() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Available skills (use read_skill tool to load):\n")

	// Sort skill names for consistent output
	names := make([]string, 0, len(l.skills))
	for name := range l.skills {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		skill := l.skills[name]
		desc := skill.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", name, desc))
	}

	return sb.String()
}

// GetSkillContent returns the full content of a skill by name.
// Returns empty string if skill is not found.
func (l *Loader) GetSkillContent(name string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if skill, exists := l.skills[name]; exists {
		return skill.Content
	}
	return ""
}

// Get returns a skill by name, or nil if not found.
func (l *Loader) Get(name string) *Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.skills[name]
}

// Count returns the number of discovered skills.
func (l *Loader) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.skills)
}
