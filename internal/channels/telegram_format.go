package channels

import (
	"regexp"
	"strings"
)

// MarkdownToTelegramHTML converts markdown-formatted text to Telegram-safe HTML.
// It handles:
// - HTML entity escaping (&, <, >)
// - **bold** -> <b>bold</b>
// - _italic_ -> <i>italic</i>
// - `code` -> <code>code</code>
// - ```code blocks``` -> <pre><code>...</code></pre>
// - [text](url) -> <a href="url">text</a>
// - # headers -> plain text (heading markers removed)
// - > blockquotes -> plain text (quote markers removed)
func MarkdownToTelegramHTML(text string) string {
	if text == "" {
		return ""
	}

	// First, handle code blocks to prevent processing markdown inside them
	// Store code blocks and replace with placeholders
	type codeBlock struct {
		content string
		isBlock bool
	}
	var codeBlocks []codeBlock
	placeholder := "\x00CODE_BLOCK_%d\x00"

	// Handle fenced code blocks (```) first
	codeBlockRegex := regexp.MustCompile("(?s)```(?:[a-zA-Z0-9]*\n?)?(.*?)```")
	text = codeBlockRegex.ReplaceAllStringFunc(text, func(match string) string {
		// Extract content between ```
		content := codeBlockRegex.FindStringSubmatch(match)
		if len(content) > 1 {
			// Escape HTML in the code content
			escapedContent := escapeHTML(strings.TrimSpace(content[1]))
			idx := len(codeBlocks)
			codeBlocks = append(codeBlocks, codeBlock{
				content: "<pre><code>" + escapedContent + "</code></pre>",
				isBlock: true,
			})
			return strings.Replace(placeholder, "%d", string(rune('0'+idx)), 1)
		}
		return match
	})

	// Handle inline code (`) - must come after code blocks
	inlineCodeRegex := regexp.MustCompile("`([^`\n]+)`")
	text = inlineCodeRegex.ReplaceAllStringFunc(text, func(match string) string {
		content := inlineCodeRegex.FindStringSubmatch(match)
		if len(content) > 1 {
			escapedContent := escapeHTML(content[1])
			idx := len(codeBlocks)
			codeBlocks = append(codeBlocks, codeBlock{
				content: "<code>" + escapedContent + "</code>",
				isBlock: false,
			})
			return strings.Replace(placeholder, "%d", string(rune('0'+idx)), 1)
		}
		return match
	})

	// Now escape HTML in the remaining text
	text = escapeHTML(text)

	// Convert markdown links [text](url) to HTML <a> tags
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	text = linkRegex.ReplaceAllString(text, `<a href="$2">$1</a>`)

	// Convert **bold** to <b>bold</b>
	boldRegex := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	text = boldRegex.ReplaceAllString(text, "<b>$1</b>")

	// Convert _italic_ to <i>italic</i>
	// Be careful not to match underscores in the middle of words
	italicRegex := regexp.MustCompile(`(?:^|[^a-zA-Z0-9])_([^_\n]+)_(?:[^a-zA-Z0-9]|$)`)
	text = italicRegex.ReplaceAllStringFunc(text, func(match string) string {
		// Find the actual italic content
		submatch := italicRegex.FindStringSubmatch(match)
		if len(submatch) > 1 {
			// Preserve leading/trailing characters that aren't part of the italic syntax
			prefix := ""
			suffix := ""
			if len(match) > 0 && match[0] != '_' {
				prefix = string(match[0])
			}
			if len(match) > 0 && match[len(match)-1] != '_' {
				suffix = string(match[len(match)-1])
			}
			return prefix + "<i>" + submatch[1] + "</i>" + suffix
		}
		return match
	})

	// Handle simpler italic case at start/end of string
	simpleItalicRegex := regexp.MustCompile(`^_([^_\n]+)_$`)
	text = simpleItalicRegex.ReplaceAllString(text, "<i>$1</i>")

	// Also handle *italic* (single asterisk)
	singleAsteriskItalic := regexp.MustCompile(`(?:^|[^*])\*([^*\n]+)\*(?:[^*]|$)`)
	text = singleAsteriskItalic.ReplaceAllStringFunc(text, func(match string) string {
		submatch := singleAsteriskItalic.FindStringSubmatch(match)
		if len(submatch) > 1 {
			prefix := ""
			suffix := ""
			if len(match) > 0 && match[0] != '*' {
				prefix = string(match[0])
			}
			if len(match) > 0 && match[len(match)-1] != '*' {
				suffix = string(match[len(match)-1])
			}
			return prefix + "<i>" + submatch[1] + "</i>" + suffix
		}
		return match
	})

	// Remove # headers (keep the text)
	headerRegex := regexp.MustCompile(`(?m)^#{1,6}\s+(.*)$`)
	text = headerRegex.ReplaceAllString(text, "$1")

	// Remove > blockquotes (keep the text)
	blockquoteRegex := regexp.MustCompile(`(?m)^>\s*(.*)$`)
	text = blockquoteRegex.ReplaceAllString(text, "$1")

	// Restore code blocks
	for i, block := range codeBlocks {
		placeholderStr := strings.Replace(placeholder, "%d", string(rune('0'+i)), 1)
		text = strings.Replace(text, placeholderStr, block.content, 1)
	}

	return text
}

// escapeHTML escapes HTML special characters.
func escapeHTML(text string) string {
	// Must escape & first to avoid double-escaping
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// StripMarkdown removes all markdown formatting and returns plain text.
// This is useful as a fallback when HTML formatting fails.
func StripMarkdown(text string) string {
	if text == "" {
		return ""
	}

	// Remove fenced code blocks markers
	codeBlockRegex := regexp.MustCompile("(?s)```(?:[a-zA-Z0-9]*\n?)?(.*?)```")
	text = codeBlockRegex.ReplaceAllString(text, "$1")

	// Remove inline code markers
	inlineCodeRegex := regexp.MustCompile("`([^`]+)`")
	text = inlineCodeRegex.ReplaceAllString(text, "$1")

	// Remove markdown links, keep text
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	text = linkRegex.ReplaceAllString(text, "$1")

	// Remove bold markers
	boldRegex := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	text = boldRegex.ReplaceAllString(text, "$1")

	// Remove italic markers (underscore)
	italicUnderscoreRegex := regexp.MustCompile(`_([^_]+)_`)
	text = italicUnderscoreRegex.ReplaceAllString(text, "$1")

	// Remove italic markers (asterisk)
	italicAsteriskRegex := regexp.MustCompile(`\*([^*]+)\*`)
	text = italicAsteriskRegex.ReplaceAllString(text, "$1")

	// Remove header markers
	headerRegex := regexp.MustCompile(`(?m)^#{1,6}\s+`)
	text = headerRegex.ReplaceAllString(text, "")

	// Remove blockquote markers
	blockquoteRegex := regexp.MustCompile(`(?m)^>\s*`)
	text = blockquoteRegex.ReplaceAllString(text, "")

	return text
}
