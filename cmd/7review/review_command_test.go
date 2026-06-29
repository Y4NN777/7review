package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRunRequestReviewCallsTool(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != "http://agent/tools/execute" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		for _, want := range []string{`"name":"request_review"`, `"provider":"github"`, `"repository":"owner/repo"`, `"pr_number":7`} {
			if !strings.Contains(string(body), want) {
				t.Fatalf("request body missing %q: %s", want, string(body))
			}
		}
		return jsonResponse(http.StatusOK, `{"name":"request_review","result":{"run_id":"owner/repo!7","status":"enqueued"}}`), nil
	})}

	var out bytes.Buffer
	if err := runRequestReview([]string{"github", "--repo", "owner/repo", "--pr", "7", "--server", "http://agent"}, &out, client); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "review enqueued for owner/repo!7") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
