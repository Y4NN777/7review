package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	if len(req.Tools) > 0 {
		payload["tools"] = anthropicToolDefinitions(req.Tools)
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

	raw, err := readModelResponse("anthropic", resp)
	if err != nil {
		return "", err
	}

	var out struct {
		Content []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
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
	var toolCalls []anthropicToolCall
	for _, block := range out.Content {
		if block.Type == "tool_use" {
			toolCalls = append(toolCalls, anthropicToolCall{Name: block.Name, Input: block.Input})
			continue
		}
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	if len(toolCalls) > 0 {
		return anthropicToolCallsEnvelope(toolCalls), nil
	}
	return "", fmt.Errorf("anthropic: no text block in response")
}

type anthropicToolCall struct {
	Name  string
	Input map[string]any
}

func anthropicToolDefinitions(tools []ToolDefinition) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		out = append(out, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": tool.InputSchema,
		})
	}
	return out
}

func anthropicToolCallsEnvelope(calls []anthropicToolCall) string {
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
		envelope.ToolRequests = append(envelope.ToolRequests, request{Name: call.Name, Input: call.Input, Reason: "provider-native tool call"})
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return `{"findings":[],"skill_coverage":[]}`
	}
	return string(data)
}
