package session

// CharsPerToken is the approximate number of characters per token
// This is a simple heuristic - actual tokenization varies by model
const CharsPerToken = 4

// TrimHistory trims session history to fit within token limits
// Uses a simple heuristic: ~4 chars per token
// It preserves the most recent messages and removes older ones
func TrimHistory(messages []Message, maxTokens int) []Message {
	if maxTokens <= 0 {
		return messages
	}

	// If no messages, return empty
	if len(messages) == 0 {
		return messages
	}

	// Calculate total tokens
	totalTokens := EstimateTokens(messages)

	// If within limit, return all messages
	if totalTokens <= maxTokens {
		result := make([]Message, len(messages))
		copy(result, messages)
		return result
	}

	// Trim from the beginning (oldest messages) until we fit
	// Keep at least one message
	result := make([]Message, len(messages))
	copy(result, messages)

	for len(result) > 1 && EstimateTokens(result) > maxTokens {
		result = result[1:]
	}

	return result
}

// EstimateTokens estimates the token count for a slice of messages
// Uses the heuristic of ~4 characters per token
func EstimateTokens(messages []Message) int {
	totalChars := 0

	for _, msg := range messages {
		// Count role (typically 4-10 chars)
		totalChars += len(msg.Role)

		// Count content
		totalChars += len(msg.Content)

		// Count tool calls if present
		for _, tc := range msg.ToolCalls {
			totalChars += len(tc.ID)
			totalChars += len(tc.Name)
			totalChars += len(tc.Arguments)
		}

		// Count tool call ID and name for tool results
		totalChars += len(msg.ToolCallID)
		totalChars += len(msg.Name)

		// Add overhead for JSON structure (approximate)
		// Each message has formatting overhead
		totalChars += 20
	}

	// Convert to tokens using the heuristic
	tokens := totalChars / CharsPerToken

	// Add a small buffer for safety
	tokens += len(messages) * 4 // ~4 tokens overhead per message

	return tokens
}

// EstimateTokensForContent estimates tokens for a single string
func EstimateTokensForContent(content string) int {
	return len(content) / CharsPerToken
}

// TrimToMessageCount returns the last n messages from the history
func TrimToMessageCount(messages []Message, maxCount int) []Message {
	if maxCount <= 0 || maxCount >= len(messages) {
		result := make([]Message, len(messages))
		copy(result, messages)
		return result
	}

	start := len(messages) - maxCount
	result := make([]Message, maxCount)
	copy(result, messages[start:])
	return result
}

// TrimPreservingSystemMessages trims history but always keeps system messages
// This is useful when you have important system prompts that should never be removed
func TrimPreservingSystemMessages(messages []Message, maxTokens int) []Message {
	if maxTokens <= 0 || len(messages) == 0 {
		return messages
	}

	// Separate system messages from others
	var systemMsgs []Message
	var otherMsgs []Message

	for _, msg := range messages {
		if msg.Role == "system" {
			systemMsgs = append(systemMsgs, msg)
		} else {
			otherMsgs = append(otherMsgs, msg)
		}
	}

	// Calculate tokens used by system messages
	systemTokens := EstimateTokens(systemMsgs)

	// If system messages alone exceed the limit, just return them
	if systemTokens >= maxTokens {
		return systemMsgs
	}

	// Calculate remaining tokens for other messages
	remainingTokens := maxTokens - systemTokens

	// Trim other messages to fit
	trimmedOthers := TrimHistory(otherMsgs, remainingTokens)

	// Combine: system messages first, then trimmed others
	result := make([]Message, 0, len(systemMsgs)+len(trimmedOthers))
	result = append(result, systemMsgs...)
	result = append(result, trimmedOthers...)

	return result
}
