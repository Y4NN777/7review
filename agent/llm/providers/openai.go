package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultOpenAIBase = "https://api.openai.com"

type OpenAI struct {
	apiKey  string
	baseURL string
	name    string
}

func NewOpenAI(apiKey, baseURL string) *OpenAI {
	if baseURL == "" {
		baseURL = defaultOpenAIBase
	}
	return &OpenAI{apiKey: apiKey, baseURL: baseURL, name: "openai"}
}

// NewOpenAICompat creates a provider for any OpenAI-compatible endpoint:
// Together AI, Groq, vLLM, LM Studio, etc.
// Just point baseURL at the server and set the right model name.
func NewOpenAICompat(apiKey, baseURL string) *OpenAI {
	return &OpenAI{apiKey: apiKey, baseURL: baseURL, name: "openai_compat"}
}

func (o *OpenAI) Name() string { return o.name }

func (o *OpenAI) Complete(ctx context.Context, req LLMRequest) (string, error) {
	payload := map[string]any{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserMessage},
		},
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		o.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%s: build request: %w", o.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%s: http: %w", o.name, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("%s: decode: %w", o.name, err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("%s: API error: %s", o.name, out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%s: empty choices in response", o.name)
	}
	return out.Choices[0].Message.Content, nil
}
