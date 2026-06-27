package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOllamaEmbedCallsEmbeddingsEndpoint(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/embeddings" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "nomic-embed-text:latest" || payload["prompt"] != "review memory query" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"embedding":[0.1,0.2,0.3]}`)),
		}, nil
	})
	defer restore()

	embedding, err := NewOllama("http://ollama.test").Embed(context.Background(), EmbeddingRequest{
		Model: "nomic-embed-text:latest",
		Input: "review memory query",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(embedding) != 3 || embedding[0] != 0.1 || embedding[2] != 0.3 {
		t.Fatalf("unexpected embedding: %#v", embedding)
	}
}

func TestOllamaEmbedRejectsEmptyEmbedding(t *testing.T) {
	restore := stubHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"embedding":[]}`)),
		}, nil
	})
	defer restore()

	_, err := NewOllama("http://ollama.test").Embed(context.Background(), EmbeddingRequest{
		Model: "nomic-embed-text:latest",
		Input: "query",
	})
	if err == nil {
		t.Fatal("expected empty embedding error")
	}
}
