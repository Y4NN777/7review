package providers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxModelResponseBytes int64 = 8 << 20

func readModelResponse(provider string, resp *http.Response) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("%s: nil HTTP response", provider)
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxModelResponseBytes+1))
	if int64(len(raw)) > maxModelResponseBytes {
		return nil, fmt.Errorf("%s: response exceeds %d bytes", provider, maxModelResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet := strings.TrimSpace(string(raw))
		if len(snippet) > 4096 {
			snippet = snippet[:4096]
		}
		return nil, fmt.Errorf("%s: API error: %s: %s", provider, resp.Status, snippet)
	}
	return raw, nil
}
