package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/config", "/tmp/workspace")

	if m.cacheDir != "/tmp/config/cache/skills-repo" {
		t.Errorf("expected cacheDir to be /tmp/config/cache/skills-repo, got %s", m.cacheDir)
	}

	if m.workspaceDir != "/tmp/workspace/skills" {
		t.Errorf("expected workspaceDir to be /tmp/workspace/skills, got %s", m.workspaceDir)
	}

	if m.repoURL != DefaultSkillsRepo {
		t.Errorf("expected repoURL to be %s, got %s", DefaultSkillsRepo, m.repoURL)
	}
}

func TestSetRepoURL(t *testing.T) {
	m := NewManager("/tmp/config", "/tmp/workspace")
	customURL := "https://github.com/custom/repo"
	m.SetRepoURL(customURL)

	if m.repoURL != customURL {
		t.Errorf("expected repoURL to be %s, got %s", customURL, m.repoURL)
	}
}

func TestIsCached(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, tmpDir)

	// Should not be cached initially
	if m.IsCached() {
		t.Error("expected IsCached to be false initially")
	}

	// Create fake .git directory
	gitDir := filepath.Join(m.cacheDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}

	// Should be cached now
	if !m.IsCached() {
		t.Error("expected IsCached to be true after creating .git dir")
	}
}

func TestInstallAndUninstall(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configDir := filepath.Join(tmpDir, "config")
	workspaceDir := filepath.Join(tmpDir, "workspace")

	m := NewManager(configDir, workspaceDir)

	// Create a fake cached skill
	skillCacheDir := filepath.Join(m.cacheDir, "test-skill")
	if err := os.MkdirAll(skillCacheDir, 0755); err != nil {
		t.Fatalf("failed to create skill cache dir: %v", err)
	}

	skillContent := `# Test Skill

A test skill for unit testing.

## Usage

This is a test skill.
`
	skillFile := filepath.Join(skillCacheDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Manually add to available
	m.available["test-skill"] = &AvailableSkill{
		Name:        "test-skill",
		Title:       "Test Skill",
		Description: "A test skill for unit testing.",
		Path:        skillFile,
	}

	// Test install
	if err := m.Install("test-skill"); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Check if skill is installed
	if !m.IsInstalled("test-skill") {
		t.Error("expected skill to be installed")
	}

	// Check installed file exists
	installedSkillFile := filepath.Join(m.workspaceDir, "test-skill", "SKILL.md")
	if _, err := os.Stat(installedSkillFile); os.IsNotExist(err) {
		t.Error("expected installed SKILL.md to exist")
	}

	// Test ListInstalled
	installed, err := m.ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled failed: %v", err)
	}
	if len(installed) != 1 || installed[0] != "test-skill" {
		t.Errorf("expected [test-skill], got %v", installed)
	}

	// Test uninstall
	if err := m.Uninstall("test-skill"); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	// Check if skill is uninstalled
	if m.IsInstalled("test-skill") {
		t.Error("expected skill to be uninstalled")
	}

	// ListInstalled should be empty
	installed, err = m.ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled failed: %v", err)
	}
	if len(installed) != 0 {
		t.Errorf("expected empty list, got %v", installed)
	}
}

func TestInstallNonExistent(t *testing.T) {
	m := NewManager("/tmp/config", "/tmp/workspace")

	err := m.Install("non-existent-skill")
	if err == nil {
		t.Error("expected error when installing non-existent skill")
	}
}

func TestUninstallNonExistent(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, tmpDir)

	err = m.Uninstall("non-existent-skill")
	if err == nil {
		t.Error("expected error when uninstalling non-existent skill")
	}
}

func TestDiscoverAvailable(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, tmpDir)

	// Create fake cache with .git and skills
	gitDir := filepath.Join(m.cacheDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}

	// Create skill with category
	categorySkillDir := filepath.Join(m.cacheDir, "category1", "skill1")
	if err := os.MkdirAll(categorySkillDir, 0755); err != nil {
		t.Fatalf("failed to create category skill dir: %v", err)
	}
	skillContent1 := `# Skill One

First skill for testing.
`
	if err := os.WriteFile(filepath.Join(categorySkillDir, "SKILL.md"), []byte(skillContent1), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Create skill without category
	simpleSkillDir := filepath.Join(m.cacheDir, "skill2")
	if err := os.MkdirAll(simpleSkillDir, 0755); err != nil {
		t.Fatalf("failed to create simple skill dir: %v", err)
	}
	skillContent2 := `# Skill Two

Second skill for testing.
`
	if err := os.WriteFile(filepath.Join(simpleSkillDir, "SKILL.md"), []byte(skillContent2), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Discover
	if err := m.DiscoverAvailable(); err != nil {
		t.Fatalf("DiscoverAvailable failed: %v", err)
	}

	// Check discovered skills
	available := m.ListAvailable()
	if len(available) != 2 {
		t.Errorf("expected 2 skills, got %d", len(available))
	}

	// Check skill1 has category
	skill1 := m.GetAvailable("skill1")
	if skill1 == nil {
		t.Fatal("expected skill1 to be discovered")
	}
	if skill1.Category != "category1" {
		t.Errorf("expected skill1 category to be category1, got %s", skill1.Category)
	}
	if skill1.Title != "Skill One" {
		t.Errorf("expected skill1 title to be 'Skill One', got %s", skill1.Title)
	}

	// Check skill2 has no category
	skill2 := m.GetAvailable("skill2")
	if skill2 == nil {
		t.Fatal("expected skill2 to be discovered")
	}
	if skill2.Category != "" {
		t.Errorf("expected skill2 category to be empty, got %s", skill2.Category)
	}
}

func TestInstallMultiple(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, tmpDir)

	// Create two cached skills
	for _, name := range []string{"skill-a", "skill-b"} {
		skillDir := filepath.Join(m.cacheDir, name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create skill dir: %v", err)
		}
		skillFile := filepath.Join(skillDir, "SKILL.md")
		content := "# " + name + "\n\nDescription."
		if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write SKILL.md: %v", err)
		}
		m.available[name] = &AvailableSkill{
			Name: name,
			Path: skillFile,
		}
	}

	// Install multiple (including one that doesn't exist)
	results := m.InstallMultiple([]string{"skill-a", "skill-b", "non-existent"})

	if results["skill-a"] != nil {
		t.Errorf("expected skill-a to install successfully, got error: %v", results["skill-a"])
	}
	if results["skill-b"] != nil {
		t.Errorf("expected skill-b to install successfully, got error: %v", results["skill-b"])
	}
	if results["non-existent"] == nil {
		t.Error("expected non-existent skill to fail")
	}

	// Verify installations
	if !m.IsInstalled("skill-a") {
		t.Error("expected skill-a to be installed")
	}
	if !m.IsInstalled("skill-b") {
		t.Error("expected skill-b to be installed")
	}
}

func TestCopyDir(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	// Create source structure
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatalf("failed to create src subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	// Copy
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify
	content1, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if err != nil || string(content1) != "content1" {
		t.Error("file1.txt not copied correctly")
	}

	content2, err := os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt"))
	if err != nil || string(content2) != "content2" {
		t.Error("subdir/file2.txt not copied correctly")
	}
}
