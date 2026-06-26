package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultAnthropicBase = "https://api.anthropic.com"
const anthropicVersion = "2023-06-01"

type Anthropic struct {
	apiKey  string
	baseURL string
}

func NewAnthropic(apiKey, baseURL string) *Anthropic {
	if baseURL == "" {
		baseURL = defaultAnthropicBase
	}
	return &Anthropic{apiKey: apiKey, baseURL: baseURL}
}

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) Complete(ctx context.Context, req LLMRequest) (string, error) {
	payload := map[string]any{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		"system":     req.SystemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": req.UserMessage},
		},
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("anthropic: http: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("anthropic: decode: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("anthropic: API error: %s", out.Error.Message)
	}
	for _, block := range out.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic: no text block in response")
}
