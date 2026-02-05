package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/hkuds/ubot/internal/skills"
)

// ReadSkillTool reads the content of a skill by name.
type ReadSkillTool struct {
	BaseTool
	loader *skills.Loader
}

// NewReadSkillTool creates a new ReadSkillTool with the given skills loader.
func NewReadSkillTool(loader *skills.Loader) *ReadSkillTool {
	return &ReadSkillTool{
		BaseTool: NewBaseTool(
			"read_skill",
			"Read the content of a skill to learn its capabilities and usage. Use this to load skill-specific instructions and tool descriptions. Call list_skills first to see available skills.",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "The name of the skill to read (e.g., 'code-review', 'github').",
					},
				},
				"required": []string{"name"},
			},
		),
		loader: loader,
	}
}

// Execute reads and returns the skill content.
func (t *ReadSkillTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	name, err := GetStringParam(params, "name")
	if err != nil {
		return "", fmt.Errorf("read_skill: %w", err)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("read_skill: name cannot be empty")
	}

	skill, err := t.loader.Load(name)
	if err != nil {
		return "", fmt.Errorf("read_skill: %w", err)
	}

	return skill.Content, nil
}

// ListSkillsTool lists all available skills.
type ListSkillsTool struct {
	BaseTool
	loader *skills.Loader
}

// NewListSkillsTool creates a new ListSkillsTool with the given skills loader.
func NewListSkillsTool(loader *skills.Loader) *ListSkillsTool {
	return &ListSkillsTool{
		BaseTool: NewBaseTool(
			"list_skills",
			"List all available skills with their names and brief descriptions. Use this to discover what skills are available before using read_skill.",
			map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		),
		loader: loader,
	}
}

// Execute lists all available skills.
func (t *ListSkillsTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	// Ensure skills are discovered
	if err := t.loader.Discover(); err != nil {
		return "", fmt.Errorf("list_skills: failed to discover skills: %w", err)
	}

	names := t.loader.List()
	if len(names) == 0 {
		return "No skills available.", nil
	}

	var sb strings.Builder
	sb.WriteString("Available skills:\n\n")

	for _, name := range names {
		skill := t.loader.Get(name)
		if skill == nil {
			continue
		}

		desc := skill.Description
		if len(desc) > 100 {
			desc = desc[:97] + "..."
		}

		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, desc))

		if len(skill.Tools) > 0 {
			sb.WriteString(fmt.Sprintf("  Tools: %s\n", strings.Join(skill.Tools, ", ")))
		}
	}

	return sb.String(), nil
}
