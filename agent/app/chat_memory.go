package app

import (
	"context"
	"strings"
	"time"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/llm/providers"
	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/tools"
)

func (s *Server) operatorMemoryBlock(ctx context.Context, message string) string {
	memory := s.operatorMemoryStore()
	if memory == nil {
		return "retrieval: unavailable\nreason: MemPalace memory store is not configured"
	}
	recall, err := memory.Recall(ctx, review.Request{
		Provider:    "operator",
		ProjectID:   "local",
		Repository:  "operator-chat",
		ChangeID:    "chat",
		Title:       message,
		Description: message,
	})
	if err != nil {
		return "retrieval: unavailable\nreason: " + err.Error()
	}
	return renderOperatorMemoryRecall(recall)
}

func (s *Server) operatorMemoryStore() pipeline.MemoryStore {
	if s == nil {
		return nil
	}
	if s.cfg != nil && strings.TrimSpace(s.cfg.MemPalaceURL) != "" {
		store := tools.NewMemPalaceStore(s.cfg.MemPalaceURL, time.Duration(s.cfg.MemPalaceTimeout)*time.Millisecond)
		if s.cfg.EmbeddingModel != "" && s.cfg.OllamaBaseURL != "" {
			store.EmbeddingModel = s.cfg.EmbeddingModel
			store.Embedder = providers.NewOllama(s.cfg.OllamaBaseURL)
			store.EmbedQueries = true
		}
		return store
	}
	if s.pipeline != nil {
		return s.pipeline.Memory
	}
	return nil
}

func reviewMemoryStore(cfg *config.Config) *tools.MemPalaceStore {
	store := tools.NewMemPalaceStore(cfg.MemPalaceURL, time.Duration(cfg.MemPalaceTimeout)*time.Millisecond)
	if cfg.EmbeddingModel != "" && cfg.OllamaBaseURL != "" {
		store.EmbeddingModel = cfg.EmbeddingModel
		store.Embedder = providers.NewOllama(cfg.OllamaBaseURL)
		store.EmbedWrites = true
	}
	return store
}
