package providers

const defaultDeepSeekBase = "https://api.deepseek.com"

func NewDeepSeek(apiKey, baseURL string) *OpenAI {
	if baseURL == "" {
		baseURL = defaultDeepSeekBase
	}
	return NewOpenAICompatible("deepseek", apiKey, baseURL)
}
