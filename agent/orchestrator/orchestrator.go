package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/Y4NN777/7review/agent/review"
	diffanalyzer "github.com/Y4NN777/7review/agent/subagents/diff_analyzer"
)

// Orchestrator is the multi-LLM coordinator.
// It knows nothing about code review logic — it only knows:
//   - which role maps to which provider chain
//   - how to fan out parallel calls and merge results
//   - how to walk the fallback chain on failure
type Orchestrator struct {
	cfg       *OrchestratorConfig
	providers map[string]LLMProvider // keyed by provider name
}

// NewOrchestrator builds an Orchestrator from an OrchestratorConfig.
// All providers referenced in the config must be registered.
func NewOrchestrator(cfg *OrchestratorConfig, providers map[string]LLMProvider) *Orchestrator {
	return &Orchestrator{cfg: cfg, providers: providers}
}

// Complete sends a request under a given role.
// Tries Primary, then each Fallback in order until one succeeds.
// Records the winning provider into ctx (ReviewContext).
func (o *Orchestrator) Complete(
	ctx context.Context,
	rc *review.Context,
	role ModelRole,
	step string,
	systemPrompt, userMessage string,
) (string, error) {
	roleCfg, ok := o.cfg.Roles[role]
	if !ok {
		return "", fmt.Errorf("orchestrator: unknown role %q", role)
	}

	chain := append([]ModelSpec{roleCfg.Primary}, roleCfg.Fallbacks...)

	var lastErr error
	for _, spec := range chain {
		provider, ok := o.providers[spec.Provider]
		if !ok {
			log.Printf("[orchestrator] %s: provider %q not registered, skipping", step, spec.Provider)
			continue
		}

		log.Printf("[orchestrator] %s: trying %s/%s", step, spec.Provider, spec.Model)
		result, err := provider.Complete(ctx, LLMRequest{
			Model:        spec.Model,
			SystemPrompt: systemPrompt,
			UserMessage:  userMessage,
			MaxTokens:    roleCfg.MaxTokens,
		})
		if err != nil {
			log.Printf("[orchestrator] %s: %s/%s failed: %v — trying fallback", step, spec.Provider, spec.Model, err)
			lastErr = err
			continue
		}

		rc.RecordProvider(step, fmt.Sprintf("%s/%s", spec.Provider, spec.Model))
		return result, nil
	}

	return "", fmt.Errorf("orchestrator: %s: all providers failed, last error: %w", step, lastErr)
}

// CompleteParallel fans out the review across multiple file batches,
// potentially routing batches to different providers concurrently.
//
// Routing strategy:
//   - Simple files (token count < threshold) → cheapest available provider
//   - Complex files (token count >= threshold) → primary reasoner
//
// All results are collected into rc.AddFindings() in a thread-safe way.
func (o *Orchestrator) CompleteParallel(
	ctx context.Context,
	rc *review.Context,
	systemPrompt string,
	batches [][]diffanalyzer.FileDiff,
	buildUserMsg func(batch []diffanalyzer.FileDiff) string,
) error {
	roleCfg, ok := o.cfg.Roles[RoleReasoner]
	if !ok {
		return errors.New("orchestrator: parallel: reasoner role not configured")
	}

	if !roleCfg.Parallel || len(batches) == 1 {
		// Serial fallback — either parallel disabled or only one batch.
		result, err := o.Complete(ctx, rc, RoleReasoner, "step5", systemPrompt, buildUserMsg(batches[0]))
		if err != nil {
			return err
		}
		rc.AddFindings(result)
		return nil
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

	// Build the provider chain once — shared across goroutines (providers are stateless).
	chain := append([]ModelSpec{roleCfg.Primary}, roleCfg.Fallbacks...)

	for i, batch := range batches {
		wg.Add(1)
		go func(batchIdx int, b []diffanalyzer.FileDiff) {
			defer wg.Done()

			// Route: pick spec based on batch complexity.
			spec := pickSpecForBatch(chain, b, 2000)
			provider, ok := o.providers[spec.Provider]
			if !ok {
				// Fallback to primary provider.
				provider = o.providers[chain[0].Provider]
				spec = chain[0]
			}

			userMsg := buildUserMsg(b)
			log.Printf("[orchestrator] step5 batch %d/%d → %s/%s",
				batchIdx+1, len(batches), spec.Provider, spec.Model)

			result, err := provider.Complete(ctx, LLMRequest{
				Model:        spec.Model,
				SystemPrompt: systemPrompt,
				UserMessage:  userMsg,
				MaxTokens:    roleCfg.MaxTokens,
			})
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("batch %d (%s/%s): %w", batchIdx+1, spec.Provider, spec.Model, err)
				}
				mu.Unlock()
				return
			}

			rc.AddFindings(result)
			rc.RecordProvider(
				fmt.Sprintf("step5_batch%d", batchIdx+1),
				fmt.Sprintf("%s/%s", spec.Provider, spec.Model),
			)
		}(i, batch)
	}

	wg.Wait()
	return firstErr
}

// pickSpecForBatch selects a ModelSpec based on the estimated token complexity
// of the batch. Below the threshold → last spec in chain (cheapest).
// At or above → first spec (most capable).
func pickSpecForBatch(chain []ModelSpec, batch []diffanalyzer.FileDiff, cheapThreshold int) ModelSpec {
	total := 0
	for _, f := range batch {
		total += f.TokenCount
	}
	if total < cheapThreshold && len(chain) > 1 {
		return chain[len(chain)-1] // cheapest = last in fallback chain
	}
	return chain[0] // primary = most capable
}
