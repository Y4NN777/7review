package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	return NewOpenAICompatible("openai_compat", apiKey, baseURL)
}

func NewOpenAICompatible(name, apiKey, baseURL string) *OpenAI {
	return &OpenAI{apiKey: apiKey, baseURL: baseURL, name: name}
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
	if len(req.Tools) > 0 {
		payload["tools"] = openAIToolDefinitions(req.Tools)
		payload["tool_choice"] = "auto"
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

	raw, err := readModelResponse(o.name, resp)
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
		return "", fmt.Errorf("%s: decode: %w", o.name, err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("%s: API error: %s", o.name, out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%s: empty choices in response", o.name)
	}
	if len(out.Choices[0].Message.ToolCalls) > 0 {
		return openAIToolCallsEnvelope(out.Choices[0].Message.ToolCalls), nil
	}
	return out.Choices[0].Message.Content, nil
}

type openAIToolCall struct {
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func openAIToolDefinitions(tools []ToolDefinition) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.InputSchema,
			},
		})
	}
	return out
}

func openAIToolCallsEnvelope(calls []openAIToolCall) string {
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
		input := map[string]any{}
		if strings.TrimSpace(call.Function.Arguments) != "" {
			_ = json.Unmarshal([]byte(call.Function.Arguments), &input)
		}
		envelope.ToolRequests = append(envelope.ToolRequests, request{Name: call.Function.Name, Input: input, Reason: "provider-native tool call"})
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return `{"findings":[],"skill_coverage":[]}`
	}
	return string(data)
}

func (o *OpenAI) Stream(ctx context.Context, req LLMRequest, emit StreamHandler) error {
	payload := map[string]any{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		"stream":     true,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserMessage},
		},
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		o.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s: build stream request: %w", o.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if o.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%s: stream http: %w", o.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s: stream API error: %s: %s", o.name, resp.Status, strings.TrimSpace(string(raw)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxStreamEventBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return fmt.Errorf("%s: decode stream chunk: %w", o.name, err)
		}
		if chunk.Error != nil {
			return fmt.Errorf("%s: stream API error: %s", o.name, chunk.Error.Message)
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				if err := emit(choice.Delta.Content); err != nil {
					return err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: read stream: %w", o.name, err)
	}
	return nil
}
