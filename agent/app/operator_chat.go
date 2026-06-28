package app

import (
	"strings"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/review"
)

func operatorChatSystemPrompt() string {
	return strings.Join([]string{
		"You are 7review's operator console assistant.",
		"Use only the provided runtime state, retrieved memory, and engineer message.",
		"Do not invent runtime state, review sessions, approvals, publishing state, provider workflows, files, or findings.",
		"If no active run is provided, do not claim to be reviewing a PR/MR.",
		"Treat runtime/setup facts as operator context, not code-review evidence, unless the engineer asks about runtime setup or a review explicitly changes deployment/config files.",
		"Keep answers concise and separate known facts from assumptions or next checks.",
	}, "\n")
}

func operatorChatUserMessage(cfg *config.Config, memoryBlock string, message string) string {
	var b strings.Builder
	b.WriteString("Runtime state:\n")
	b.WriteString(operatorRuntimeState(cfg))
	b.WriteString("\n\nRetrieved memory:\n")
	b.WriteString(firstNonEmpty(strings.TrimSpace(memoryBlock), "retrieval: unavailable"))
	b.WriteString("\n\nEngineer message:\n")
	b.WriteString(strings.TrimSpace(message))
	return b.String()
}

func operatorRuntimeState(cfg *config.Config) string {
	if cfg == nil {
		return "provider: unknown\nreview_model: unknown\nchat_model: unknown\nembedding_model: unknown"
	}
	return strings.Join([]string{
		"provider: " + firstNonEmpty(cfg.Provider, "unknown"),
		"review_model: " + firstNonEmpty(cfg.ReviewModel, "unknown"),
		"chat_model: " + firstNonEmpty(cfg.SmallModel, "unknown"),
		"embedding_model: " + firstNonEmpty(cfg.EmbeddingModel, "not configured"),
		"orchestrator_config: " + firstNonEmpty(cfg.OrchestratorConfigPath, "env single-provider mode"),
	}, "\n")
}

func deterministicOperatorAnswer(cfg *config.Config, input string) (string, bool) {
	text := strings.ToLower(strings.TrimSpace(input))
	if cfg == nil {
		cfg = &config.Config{}
	}
	switch {
	case containsAny(text, "what is your role", "what's your role", "your role here", "role here in this system", "what are you doing here"):
		return strings.Join([]string{
			"I am the operator console assistant for 7review.",
			"In this no-run chat, I help operate the running server and inspect runtime state.",
			"I am not currently reviewing a PR/MR because no active run is selected.",
			"Use `/sessions` to list runs or `/status` and `/providers` to inspect runtime state.",
		}, "\n"), true
	case containsAny(text, "who created you", "who made you", "are you codex", "are you openai", "are you claude", "are you opencode"):
		return "I am 7review running on the configured model provider. I should not claim to be Codex, OpenAI, Claude, OpenCode, Qwen, Ollama, or any provider/harness.", true
	case containsAny(text, "what kind of model", "what model are you", "which model", "your model", "models are you"):
		return strings.Join([]string{
			"I am 7review's operator console assistant, backed by the configured model routing.",
			"Provider: " + firstNonEmpty(cfg.Provider, "unknown"),
			"Review model: " + firstNonEmpty(cfg.ReviewModel, "unknown"),
			"Formatter/chat model: " + firstNonEmpty(cfg.SmallModel, "unknown"),
			"Embedding model: " + firstNonEmpty(cfg.EmbeddingModel, "not configured"),
			"Orchestrator config: " + firstNonEmpty(cfg.OrchestratorConfigPath, "env single-provider mode"),
			"Use `/providers` for live runtime status.",
		}, "\n"), true
	case containsAny(text, "context window", "context size", "context length", "token window"):
		return strings.Join([]string{
			"7review does not treat a diff hunk as the model context window.",
			"It builds review context from SCM diff, selected corpus, recalled memory, and Headroom compression.",
			"The exact context window depends on the configured provider/model.",
			"Provider: " + firstNonEmpty(cfg.Provider, "unknown"),
			"Review model: " + firstNonEmpty(cfg.ReviewModel, "unknown"),
			"Formatter/chat model: " + firstNonEmpty(cfg.SmallModel, "unknown"),
			"Embedding model: " + firstNonEmpty(cfg.EmbeddingModel, "not configured"),
		}, "\n"), true
	default:
		return "", false
	}
}

func renderOperatorMemoryRecall(recall review.MemoryRecall) string {
	var lines []string
	lines = append(lines, "retrieval: mempalace")
	lines = appendMemoryRecallSection(lines, "conventions", recall.Conventions)
	lines = appendMemoryRecallSection(lines, "decisions", recall.Decisions)
	lines = appendMemoryRecallSection(lines, "history", recall.History)
	if len(lines) == 1 {
		lines = append(lines, "no matching memory")
	}
	return strings.Join(lines, "\n")
}

func appendMemoryRecallSection(lines []string, label string, values []string) []string {
	if len(values) == 0 {
		return lines
	}
	lines = append(lines, label+":")
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		lines = append(lines, "- "+truncatePromptEventMessage(value))
	}
	return lines
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
