package providers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOpenAIStream_EmitsSSEChunks(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body := strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"hello "}}]}`,
			"",
			`data: {"choices":[{"delta":{"content":"engineer"}}]}`,
			"",
			"data: [DONE]",
			"",
		}, "\n")
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	defer restore()

	provider := NewOpenAI("token", "http://openai.test")
	var chunks []string
	err := provider.Stream(context.Background(), LLMRequest{Model: "test", MaxTokens: 64}, func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(chunks, ""); got != "hello engineer" {
		t.Fatalf("unexpected stream: %q", got)
	}
}

func TestOpenAIStream_AcceptsLargeSSEChunk(t *testing.T) {
	large := strings.Repeat("x", 128*1024)
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		body := `data: {"choices":[{"delta":{"content":"` + large + `"}}]}` + "\n\ndata: [DONE]\n\n"
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	defer restore()

	provider := NewOpenAI("token", "http://openai.test")
	var chunks []string
	err := provider.Stream(context.Background(), LLMRequest{Model: "test", MaxTokens: 64}, func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(chunks, ""); got != large {
		t.Fatalf("unexpected large stream length: got %d want %d", len(got), len(large))
	}
}

func TestOllamaStream_EmitsJSONLineChunks(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body := strings.Join([]string{
			`{"message":{"content":"review "}}`,
			`{"message":{"content":"ready"},"done":true}`,
			"",
		}, "\n")
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	defer restore()

	provider := NewOllama("http://ollama.test")
	var chunks []string
	err := provider.Stream(context.Background(), LLMRequest{Model: "test", MaxTokens: 64}, func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(chunks, ""); got != "review ready" {
		t.Fatalf("unexpected stream: %q", got)
	}
}

func TestOllamaStream_AcceptsLargeJSONLineChunk(t *testing.T) {
	large := strings.Repeat("x", 128*1024)
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		body := `{"message":{"content":"` + large + `"},"done":true}` + "\n"
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	defer restore()

	provider := NewOllama("http://ollama.test")
	var chunks []string
	err := provider.Stream(context.Background(), LLMRequest{Model: "test", MaxTokens: 64}, func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(chunks, ""); got != large {
		t.Fatalf("unexpected large stream length: got %d want %d", len(got), len(large))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func stubHTTPClient(t *testing.T, fn roundTripFunc) func() {
	t.Helper()
	original := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: fn}
	return func() { http.DefaultClient = original }
}
