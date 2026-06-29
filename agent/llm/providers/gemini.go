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
	if len(req.Tools) > 0 {
		payload["tools"] = geminiToolDefinitions(req.Tools)
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
					Text         string              `json:"text"`
					FunctionCall *geminiFunctionCall `json:"functionCall"`
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
	var calls []geminiFunctionCall
	for _, part := range out.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			calls = append(calls, *part.FunctionCall)
		}
	}
	if len(calls) > 0 {
		return geminiToolCallsEnvelope(calls), nil
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

func geminiToolDefinitions(tools []ToolDefinition) []map[string]any {
	declarations := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		declarations = append(declarations, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  stripUnsupportedGeminiSchemaFields(tool.InputSchema),
		})
	}
	return []map[string]any{{"functionDeclarations": declarations}}
}

func stripUnsupportedGeminiSchemaFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			if key == "additionalProperties" {
				continue
			}
			out[key] = stripUnsupportedGeminiSchemaFields(child)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, child := range typed {
			out = append(out, stripUnsupportedGeminiSchemaFields(child))
		}
		return out
	default:
		return value
	}
}

func geminiToolCallsEnvelope(calls []geminiFunctionCall) string {
	type request struct {
		Name   string         `json:"name"`
		Input  map[string]any `json:"input,omitempty"`
		Reason string         `json:"reason,omitempty"`
	}
	envelope := struct {
		ToolRequests  []request `json:"tool_requests"`
		Findings      []any     `json:"findings"`
		SkillCoverage []any     `json:"skill_coverage"`
	}{Findings: []any{}, SkillCoverage: []any{}}
	for _, call := range calls {
		envelope.ToolRequests = append(envelope.ToolRequests, request{Name: call.Name, Input: call.Args, Reason: "provider-native tool call"})
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return `{"findings":[],"skill_coverage":[]}`
	}
	return string(data)
}
