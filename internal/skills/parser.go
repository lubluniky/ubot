package skills

import (
	"os"
	"regexp"
	"strings"
)

// ParseSkillFile parses a SKILL.md file and returns a Skill struct.
// The parser extracts:
// - Title from the first # heading
// - Description from the first paragraph after the title
// - Tool names from the ## Tools section
// - AlwaysLoad flag from <!-- always-load --> comment
func ParseSkillFile(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseSkillContent(string(content), path)
}

// ParseSkillContent parses skill content from a string.
func ParseSkillContent(content string, path string) (*Skill, error) {
	skill := &Skill{
		Content:    content,
		Path:       path,
		Tools:      []string{},
		AlwaysLoad: false,
	}

	lines := strings.Split(content, "\n")

	// Parse the content
	skill.Title = parseTitle(lines)
	skill.Description = parseDescription(lines)
	skill.Tools = parseTools(lines)
	skill.AlwaysLoad = parseAlwaysLoad(content)

	return skill, nil
}

// parseTitle extracts the title from the first # heading.
func parseTitle(lines []string) string {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
	}
	return ""
}

// parseDescription extracts the first paragraph after the title heading.
func parseDescription(lines []string) string {
	foundTitle := false
	var descLines []string
	inDescription := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip until we find the title
		if !foundTitle {
			if strings.HasPrefix(trimmed, "# ") {
				foundTitle = true
			}
			continue
		}

		// Skip empty lines before description starts
		if !inDescription {
			if trimmed == "" {
				continue
			}
			// Check if this is another heading (end of description area)
			if strings.HasPrefix(trimmed, "#") {
				break
			}
			inDescription = true
		}

		// Check for end of paragraph
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			break
		}

		descLines = append(descLines, trimmed)
	}

	return strings.Join(descLines, " ")
}

// parseTools extracts tool names from the ## Tools section.
// It looks for markdown list items with backtick-wrapped tool names.
// Example: - `tool_name`: description
func parseTools(lines []string) []string {
	var tools []string
	inToolsSection := false

	// Regex to match tool names in backticks at the start of list items
	// Matches: - `tool_name` or - `tool_name`: description
	toolPattern := regexp.MustCompile("^\\s*[-*]\\s*`([^`]+)`")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for ## Tools or ## tools heading
		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			if heading == "tools" {
				inToolsSection = true
				continue
			} else if inToolsSection {
				// Another section started
				break
			}
		}

		if !inToolsSection {
			continue
		}

		// Look for tool patterns in list items
		matches := toolPattern.FindStringSubmatch(line)
		if len(matches) >= 2 {
			tools = append(tools, matches[1])
		}
	}

	return tools
}

// parseAlwaysLoad checks if the content contains an always-load directive.
// The directive can be in the format:
// - <!-- always-load -->
// - <!-- always_load -->
// - <!-- alwaysload -->
func parseAlwaysLoad(content string) bool {
	lowerContent := strings.ToLower(content)

	patterns := []string{
		"<!-- always-load -->",
		"<!-- always_load -->",
		"<!-- alwaysload -->",
		"<!--always-load-->",
		"<!--always_load-->",
		"<!--alwaysload-->",
	}

	for _, pattern := range patterns {
		if strings.Contains(lowerContent, pattern) {
			return true
		}
	}

	return false
}
