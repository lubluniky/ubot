package providers

// Default model for MiniMax.
const DefaultMiniMaxModel = "MiniMax-M2.5"

// MiniMaxBaseURL returns the API base URL for the given region.
func MiniMaxBaseURL(region string) string {
	if region == "cn" {
		return "https://api.minimaxi.com"
	}
	return "https://api.minimax.io"
}

// NewMiniMaxProvider creates a new MiniMax provider.
// It wraps an OpenAIProvider pointed at the regional MiniMax endpoint.
func NewMiniMaxProvider(apiKey, region, model string) *OpenAIProvider {
	if model == "" {
		model = DefaultMiniMaxModel
	}
	baseURL := MiniMaxBaseURL(region)
	return NewOpenAIProvider("minimax", apiKey, baseURL+"/v1", model)
}
