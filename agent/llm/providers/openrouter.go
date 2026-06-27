package providers

const defaultOpenRouterBase = "https://openrouter.ai/api"

func NewOpenRouter(apiKey, baseURL string) *OpenAI {
	if baseURL == "" {
		baseURL = defaultOpenRouterBase
	}
	return NewOpenAICompatible("openrouter", apiKey, baseURL)
}
