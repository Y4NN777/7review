package operator

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClientExecuteToolDecodesResultAndAddsAuth(t *testing.T) {
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	client := NewClient("http://agent", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != "http://agent/tools/execute" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer agent-token" || req.Header.Get("X-7review-Token") != "agent-token" {
			t.Fatalf("missing auth headers: %#v", req.Header)
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"name":"get_selected_context"`) || !strings.Contains(string(body), `"run":"owner/repo!7"`) {
			t.Fatalf("unexpected tool request body: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{"name":"get_selected_context","result":{"run":"owner/repo!7","evidence_manifest":[{"source":"docs/SRS.md","heading_or_key":"REQ-12","score":10}]}}`), nil
	})})

	var selected SelectedContext
	if err := client.ExecuteTool("get_selected_context", map[string]any{"run": "owner/repo!7"}, &selected); err != nil {
		t.Fatal(err)
	}
	if selected.Run != "owner/repo!7" || len(selected.Evidence) != 1 || selected.Evidence[0].Source != "docs/SRS.md" {
		t.Fatalf("unexpected selected context: %#v", selected)
	}
}

func TestClientGetJSONDecodesEndpoint(t *testing.T) {
	client := NewClient("", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != "http://agent/tools" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonResponse(http.StatusOK, `[{"name":"list_runs"}]`), nil
	})})

	var tools []struct {
		Name string `json:"name"`
	}
	if err := client.GetJSON("http://agent/tools", &tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "list_runs" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
