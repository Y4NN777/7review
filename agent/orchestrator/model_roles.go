package orchestrator

// ModelRole is a semantic identifier for what a model needs to do.
// Steps declare a role; the orchestrator resolves it to a real provider+model.
type ModelRole string

const (
	// RoleReasoner is for deep analysis tasks requiring large context and
	// strong reasoning. Used by Step 5 (Review Agent).
	RoleReasoner ModelRole = "reasoner"

	// RoleFormatter is for structured output tasks: JSON, Markdown formatting.
	// Used by Step 6 (Report Generator).
	RoleFormatter ModelRole = "formatter"

	// RoleEmbedder is for producing vector embeddings (memory search in Step 3).
	// Not a chat model — handled separately by the embedding client.
	RoleEmbedder ModelRole = "embedder"
)

// ModelSpec identifies a specific model at a specific provider.
// Format: "model-name@provider", e.g. "claude-sonnet-4-6@anthropic"
type ModelSpec struct {
	Model    string
	Provider string
}

// RoleConfig is the configuration for one model role.
// The orchestrator tries Primary first, then walks Fallbacks in order.
type RoleConfig struct {
	Primary   ModelSpec
	Fallbacks []ModelSpec

	// MaxTokens caps the response for this role.
	MaxTokens int

	// Parallel controls whether Step 5 may fan out across multiple specs.
	// Only meaningful for RoleReasoner.
	Parallel bool

	// MaxParallel caps concurrent calls for a parallel role.
	MaxParallel int
}

// OrchestratorConfig maps every role to its provider chain.
// Loaded from config file or environment. Example YAML:
//
//	roles:
//	  reasoner:
//	    primary: "claude-sonnet-4-6@anthropic"
//	    fallbacks:
//	      - "gpt-4o@openai"
//	      - "qwen2.5-coder:32b@ollama"
//	    max_tokens: 4096
//	    parallel: true
//	    max_parallel: 4
//	  formatter:
//	    primary: "claude-haiku-4-5-20251001@anthropic"
//	    fallbacks:
//	      - "mistral-small-latest@mistral"
//	    max_tokens: 2048
type OrchestratorConfig struct {
	Roles map[ModelRole]RoleConfig
}

// DefaultOrchestratorConfig returns a safe default that uses a single
// provider for all roles. Override via config file in production.
func DefaultOrchestratorConfig(reviewModel, smallModel, provider string) *OrchestratorConfig {
	primary := func(model string) ModelSpec {
		return ModelSpec{Model: model, Provider: provider}
	}
	return &OrchestratorConfig{
		Roles: map[ModelRole]RoleConfig{
			RoleReasoner: {
				Primary:     primary(reviewModel),
				MaxTokens:   4096,
				Parallel:    true,
				MaxParallel: 4,
			},
			RoleFormatter: {
				Primary:   primary(smallModel),
				MaxTokens: 2048,
			},
		},
	}
}
