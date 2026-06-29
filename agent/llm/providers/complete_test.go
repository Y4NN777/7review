package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestOpenAICompleteSendsExpectedRequestAndParsesResponse(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected headers: %#v", r.Header)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "gpt-test" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		return jsonResponse(`{"choices":[{"message":{"content":"review ok"}}]}`), nil
	})
	defer restore()

	got, err := NewOpenAI("token", "http://openai.test").Complete(context.Background(), LLMRequest{
		Model:        "gpt-test",
		MaxTokens:    128,
		SystemPrompt: "system",
		UserMessage:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "review ok" {
		t.Fatalf("unexpected response %q", got)
	}
}

func TestOpenAICompleteSendsNativeToolSchemasAndParsesToolCalls(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) != 1 || payload["tool_choice"] != "auto" {
			t.Fatalf("native tools not serialized: %#v", payload)
		}
		return jsonResponse(`{"choices":[{"message":{"tool_calls":[{"function":{"name":"get_changed_files","arguments":"{\"run\":\"p!7\"}"}}]}}]}`), nil
	})
	defer restore()

	got, err := NewOpenAI("token", "http://openai.test").Complete(context.Background(), LLMRequest{
		Model:     "gpt-test",
		MaxTokens: 128,
		Tools: []ToolDefinition{{
			Name:        "get_changed_files",
			Description: "Fetch changed files.",
			InputSchema: map[string]any{"type": "object"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"tool_requests"`) || !strings.Contains(got, `"get_changed_files"`) {
		t.Fatalf("tool call was not converted to envelope: %s", got)
	}
}

func TestOpenAICompatNameAndBaseURL(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "http://compat.test/v1/chat/completions" {
			t.Fatalf("unexpected URL: %s", r.URL.String())
		}
		return jsonResponse(`{"choices":[{"message":{"content":"compat ok"}}]}`), nil
	})
	defer restore()

	provider := NewOpenAICompat("token", "http://compat.test")
	if provider.Name() != "openai_compat" {
		t.Fatalf("unexpected provider name %q", provider.Name())
	}
	got, err := provider.Complete(context.Background(), LLMRequest{Model: "m", MaxTokens: 8})
	if err != nil {
		t.Fatal(err)
	}
	if got != "compat ok" {
		t.Fatalf("unexpected response %q", got)
	}
}

func TestOpenRouterAndDeepSeekUseOpenAICompatibleTransport(t *testing.T) {
	cases := []struct {
		name     string
		provider *OpenAI
		wantURL  string
	}{
		{name: "openrouter", provider: NewOpenRouter("token", "http://openrouter.test"), wantURL: "http://openrouter.test/v1/chat/completions"},
		{name: "deepseek", provider: NewDeepSeek("token", "http://deepseek.test"), wantURL: "http://deepseek.test/v1/chat/completions"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != tc.wantURL {
					t.Fatalf("unexpected URL: %s", r.URL.String())
				}
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Fatalf("missing bearer token: %#v", r.Header)
				}
				return jsonResponse(`{"choices":[{"message":{"content":"ok"}}]}`), nil
			})
			defer restore()

			if tc.provider.Name() != tc.name {
				t.Fatalf("unexpected provider name %q", tc.provider.Name())
			}
			got, err := tc.provider.Complete(context.Background(), LLMRequest{Model: "model", MaxTokens: 8})
			if err != nil {
				t.Fatal(err)
			}
			if got != "ok" {
				t.Fatalf("unexpected response %q", got)
			}
		})
	}
}

func TestAnthropicCompleteSendsVersionAndParsesTextBlock(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "anthropic-key" || r.Header.Get("anthropic-version") != anthropicVersion {
			t.Fatalf("unexpected headers: %#v", r.Header)
		}
		return jsonResponse(`{"content":[{"type":"text","text":"anthropic ok"}]}`), nil
	})
	defer restore()

	got, err := NewAnthropic("anthropic-key", "http://anthropic.test").Complete(context.Background(), LLMRequest{Model: "claude", MaxTokens: 64})
	if err != nil {
		t.Fatal(err)
	}
	if got != "anthropic ok" {
		t.Fatalf("unexpected response %q", got)
	}
}

func TestMistralCompleteSendsBearerAndParsesChoice(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer mistral-key" {
			t.Fatalf("unexpected request: %s %#v", r.URL.String(), r.Header)
		}
		return jsonResponse(`{"choices":[{"message":{"content":"mistral ok"}}]}`), nil
	})
	defer restore()

	got, err := NewMistral("mistral-key", "http://mistral.test").Complete(context.Background(), LLMRequest{Model: "mistral", MaxTokens: 64})
	if err != nil {
		t.Fatal(err)
	}
	if got != "mistral ok" {
		t.Fatalf("unexpected response %q", got)
	}
}

func TestMistralCompleteSendsNativeToolSchemasAndParsesToolCalls(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["tools"].([]any); !ok {
			t.Fatalf("mistral tools not serialized: %#v", payload)
		}
		return jsonResponse(`{"choices":[{"message":{"tool_calls":[{"function":{"name":"get_selected_context","arguments":"{}"}}]}}]}`), nil
	})
	defer restore()

	got, err := NewMistral("mistral-key", "http://mistral.test").Complete(context.Background(), LLMRequest{
		Model: "mistral",
		Tools: []ToolDefinition{{Name: "get_selected_context", Description: "Fetch context.", InputSchema: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"tool_requests"`) || !strings.Contains(got, `"get_selected_context"`) {
		t.Fatalf("mistral tool call was not converted: %s", got)
	}
}

func TestGeminiCompleteSendsAPIKeyAndParsesCandidate(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1beta/models/gemini-test:generateContent" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			t.Fatal(err)
		}
		if values.Get("key") != "gemini-key" {
			t.Fatalf("missing gemini key in query: %s", r.URL.RawQuery)
		}
		return jsonResponse(`{"candidates":[{"content":{"parts":[{"text":"gemini ok"}]}}]}`), nil
	})
	defer restore()

	got, err := NewGemini("gemini-key", "http://gemini.test").Complete(context.Background(), LLMRequest{Model: "gemini-test", MaxTokens: 64})
	if err != nil {
		t.Fatal(err)
	}
	if got != "gemini ok" {
		t.Fatalf("unexpected response %q", got)
	}
}

func TestOllamaCompleteSendsNativeToolSchemasAndParsesToolCalls(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["tools"].([]any); !ok {
			t.Fatalf("ollama tools not serialized: %#v", payload)
		}
		return jsonResponse(`{"message":{"tool_calls":[{"function":{"name":"get_diff_summary","arguments":"{}"}}]}}`), nil
	})
	defer restore()

	got, err := NewOllama("http://ollama.test").Complete(context.Background(), LLMRequest{
		Model: "qwen",
		Tools: []ToolDefinition{{Name: "get_diff_summary", Description: "Fetch diff.", InputSchema: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"tool_requests"`) || !strings.Contains(got, `"get_diff_summary"`) {
		t.Fatalf("ollama tool call was not converted: %s", got)
	}
}

func TestGeminiCompleteSendsNativeToolSchemasAndParsesToolCalls(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1beta/models/gemini-test:generateContent" {
			t.Fatalf("unexpected Gemini tool path: %s", r.URL.Path)
		}
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			t.Fatal(err)
		}
		if values.Get("key") != "gemini-key" {
			t.Fatalf("missing Gemini API key query: %s", r.URL.RawQuery)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["tools"].([]any); !ok {
			t.Fatalf("gemini tools not serialized: %#v", payload)
		}
		rawTools := payload["tools"].([]any)
		firstTool := rawTools[0].(map[string]any)
		decls, ok := firstTool["functionDeclarations"].([]any)
		if !ok {
			t.Fatalf("Gemini function declarations missing: %#v", payload)
		}
		params := decls[0].(map[string]any)["parameters"].(map[string]any)
		if _, ok := params["additionalProperties"]; ok {
			t.Fatalf("Gemini schema still contains unsupported additionalProperties: %#v", params)
		}
		return jsonResponse(`{"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_inline_positions","args":{"run":"p!7"}}}]}}]}`), nil
	})
	defer restore()

	got, err := NewGemini("gemini-key", "http://gemini.test").Complete(context.Background(), LLMRequest{
		Model: "gemini-test",
		Tools: []ToolDefinition{{Name: "get_inline_positions", Description: "Fetch positions.", InputSchema: map[string]any{"type": "object", "additionalProperties": false}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"tool_requests"`) || !strings.Contains(got, `"get_inline_positions"`) {
		t.Fatalf("gemini tool call was not converted: %s", got)
	}
}

func TestAnthropicCompleteSendsNativeToolSchemasAndParsesToolCalls(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["tools"].([]any); !ok {
			t.Fatalf("anthropic tools not serialized: %#v", payload)
		}
		return jsonResponse(`{"content":[{"type":"tool_use","name":"get_changed_files","input":{"run":"p!7"}}]}`), nil
	})
	defer restore()

	got, err := NewAnthropic("anthropic-key", "http://anthropic.test").Complete(context.Background(), LLMRequest{
		Model: "claude",
		Tools: []ToolDefinition{{Name: "get_changed_files", Description: "Fetch files.", InputSchema: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"tool_requests"`) || !strings.Contains(got, `"get_changed_files"`) {
		t.Fatalf("anthropic tool call was not converted: %s", got)
	}
}

func TestReadModelResponseRejectsHTTPErrorBeforeDecode(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Status:     "401 Unauthorized",
		Body:       io.NopCloser(strings.NewReader("bad token")),
	}
	_, err := readModelResponse("openai", resp)
	if err == nil || !strings.Contains(err.Error(), "401 Unauthorized") || !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadModelResponseRejectsOversizedBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", int(maxModelResponseBytes)+1))),
	}
	_, err := readModelResponse("gemini", resp)
	if err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
