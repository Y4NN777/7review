package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const defaultMistralBase = "https://api.mistral.ai"

type Mistral struct {
	apiKey  string
	baseURL string
}

func NewMistral(apiKey, baseURL string) *Mistral {
	if baseURL == "" {
		baseURL = defaultMistralBase
	}
	return &Mistral{apiKey: apiKey, baseURL: baseURL}
}

func (m *Mistral) Name() string { return "mistral" }

func (m *Mistral) Complete(ctx context.Context, req LLMRequest) (string, error) {
	payload := map[string]any{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserMessage},
		},
	}
	if len(req.Tools) > 0 {
		payload["tools"] = openAIToolDefinitions(req.Tools)
		payload["tool_choice"] = "auto"
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		m.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("mistral: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("mistral: http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := readModelResponse("mistral", resp)
	if err != nil {
		return "", err
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content   string           `json:"content"`
				ToolCalls []openAIToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("mistral: decode: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("mistral: API error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("mistral: empty choices")
	}
	if len(out.Choices[0].Message.ToolCalls) > 0 {
		return openAIToolCallsEnvelope(out.Choices[0].Message.ToolCalls), nil
	}
	return out.Choices[0].Message.Content, nil
}
