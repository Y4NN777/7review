package providers

import (
	"bufio"
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
	if len(req.Tools) > 0 {
		payload["tools"] = openAIToolDefinitions(req.Tools)
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
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("ollama: decode: %w", err)
	}
	if out.Error != "" {
		return "", fmt.Errorf("ollama: API error: %s", out.Error)
	}
	if len(out.Message.ToolCalls) > 0 {
		return openAIToolCallsEnvelope(out.Message.ToolCalls), nil
	}
	return out.Message.Content, nil
}

func (o *Ollama) Stream(ctx context.Context, req LLMRequest, emit StreamHandler) error {
	payload := map[string]any{
		"model":  req.Model,
		"stream": true,
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
		return fmt.Errorf("ollama: build stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama: stream http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ollama: stream API error: %s: %s", resp.Status, string(raw))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxStreamEventBytes)
	for scanner.Scan() {
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done  bool   `json:"done"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			return fmt.Errorf("ollama: decode stream chunk: %w", err)
		}
		if chunk.Error != "" {
			return fmt.Errorf("ollama: stream API error: %s", chunk.Error)
		}
		if chunk.Message.Content != "" {
			if err := emit(chunk.Message.Content); err != nil {
				return err
			}
		}
		if chunk.Done {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("ollama: read stream: %w", err)
	}
	return nil
}

func (o *Ollama) Embed(ctx context.Context, req EmbeddingRequest) ([]float64, error) {
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Input,
	}
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: build embedding request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: embedding http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("ollama: embedding API error: %s: %s", resp.Status, string(raw))
	}
	var out struct {
		Embedding []float64 `json:"embedding"`
		Error     string    `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("ollama: decode embedding: %w", err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("ollama: embedding API error: %s", out.Error)
	}
	if len(out.Embedding) == 0 {
		return nil, fmt.Errorf("ollama: empty embedding")
	}
	return out.Embedding, nil
}
