package llm

import "context"

// LLMRequest contains the provider-neutral inputs for one chat completion.
type LLMRequest struct {
	Model        string
	SystemPrompt string
	UserMessage  string
	MaxTokens    int
	Tools        []ToolDefinition
}

// ToolDefinition is a provider-neutral native tool/function schema.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// LLMProvider is implemented by each model provider integration.
type LLMProvider interface {
	Name() string
	Complete(ctx context.Context, req LLMRequest) (string, error)
}

type StreamHandler func(delta string) error

type StreamingLLMProvider interface {
	LLMProvider
	Stream(ctx context.Context, req LLMRequest, emit StreamHandler) error
}

// EmbeddingRequest contains provider-neutral inputs for a vector embedding.
type EmbeddingRequest struct {
	Model string
	Input string
}

// Embedder is implemented by providers that can produce vector embeddings.
type Embedder interface {
	Embed(ctx context.Context, req EmbeddingRequest) ([]float64, error)
}
