package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultOllamaBase = "http://localhost:11434"

type Ollama struct {
	baseURL string
}

func NewOllama(baseURL string) *Ollama {
	if baseURL == "" {
		baseURL = defaultOllamaBase
	}
	return &Ollama{baseURL: baseURL}
}

func (o *Ollama) Name() string { return "ollama" }

func (o *Ollama) Complete(ctx context.Context, req LLMRequest) (string, error) {
	// Ollama uses /api/chat with the same OpenAI message shape.
	payload := map[string]any{
		"model":  req.Model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserMessage},
		},
		"options": map[string]int{
			"num_predict": req.MaxTokens,
		},
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama: http: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("ollama: decode: %w", err)
	}
	if out.Error != "" {
		return "", fmt.Errorf("ollama: API error: %s", out.Error)
	}
	return out.Message.Content, nil
}
