package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const defaultGeminiBase = "https://generativelanguage.googleapis.com"

type Gemini struct {
	apiKey  string
	baseURL string
}

func NewGemini(apiKey, baseURL string) *Gemini {
	if baseURL == "" {
		baseURL = defaultGeminiBase
	}
	return &Gemini{apiKey: apiKey, baseURL: baseURL}
}

func (g *Gemini) Name() string { return "gemini" }

func (g *Gemini) Complete(ctx context.Context, req LLMRequest) (string, error) {
	// Gemini uses generateContent with a system_instruction + user part.
	payload := map[string]any{
		"system_instruction": map[string]any{
			"parts": []map[string]string{{"text": req.SystemPrompt}},
		},
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": []map[string]string{{"text": req.UserMessage}},
			},
		},
		"generationConfig": map[string]int{
			"maxOutputTokens": req.MaxTokens,
		},
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		g.baseURL, req.Model, g.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gemini: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("gemini: http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := readModelResponse("gemini", resp)
	if err != nil {
		return "", err
	}

	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("gemini: decode: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("gemini: API error: %s", out.Error.Message)
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty response")
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}
