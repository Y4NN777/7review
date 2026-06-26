package providers

import "context"

// LLMRequest contains the provider-neutral inputs for one chat completion.
type LLMRequest struct {
	Model        string
	SystemPrompt string
	UserMessage  string
	MaxTokens    int
}

// LLMProvider is implemented by each model provider integration.
type LLMProvider interface {
	Name() string
	Complete(ctx context.Context, req LLMRequest) (string, error)
}
