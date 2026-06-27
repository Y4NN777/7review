package tools

import (
	"strings"
	"testing"
)

func TestDecodeToolJSONRejectsOversizedResponse(t *testing.T) {
	var out map[string]string
	err := decodeToolJSON("github", "GET", "/large", strings.NewReader(strings.Repeat("x", int(maxToolResponseBodyBytes)+1)), &out)
	if err == nil || !strings.Contains(err.Error(), "response body exceeds") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestDecodeToolJSONWrapsMalformedResponse(t *testing.T) {
	var out map[string]string
	err := decodeToolJSON("mempalace", "POST", "/recall", strings.NewReader("{bad json"), &out)
	if err == nil || !strings.Contains(err.Error(), "mempalace: POST /recall: decode response") {
		t.Fatalf("expected wrapped decode error, got %v", err)
	}
}

func TestReadToolErrorBodyIsBounded(t *testing.T) {
	body := readToolErrorBody(strings.NewReader(strings.Repeat("x", int(maxToolErrorBodyBytes)+100)))
	if len(body) != int(maxToolErrorBodyBytes) {
		t.Fatalf("expected bounded error body length %d, got %d", maxToolErrorBodyBytes, len(body))
	}
}
