package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const maxToolResponseBodyBytes int64 = 8 << 20
const maxToolErrorBodyBytes int64 = 4 << 10

func decodeToolJSON(service, method, path string, body io.Reader, out any) error {
	if out == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(body, maxToolResponseBodyBytes+1))
	if err != nil {
		return fmt.Errorf("%s: %s %s: read response: %w", service, method, path, err)
	}
	if int64(len(data)) > maxToolResponseBodyBytes {
		return fmt.Errorf("%s: %s %s: response body exceeds %d bytes", service, method, path, maxToolResponseBodyBytes)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s: %s %s: decode response: %w", service, method, path, err)
	}
	return nil
}

func readToolErrorBody(body io.Reader) string {
	data, _ := io.ReadAll(io.LimitReader(body, maxToolErrorBodyBytes))
	return strings.TrimSpace(string(data))
}
