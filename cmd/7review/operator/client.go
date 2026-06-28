package operator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Y4NN777/7review/agent/tools"
)

const defaultRequestTimeout = 15 * time.Second

type Client struct {
	ServerURL  string
	HTTPClient *http.Client
}

func NewClient(serverURL string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultRequestTimeout}
	}
	return Client{ServerURL: strings.TrimRight(serverURL, "/"), HTTPClient: httpClient}
}

func (c Client) GetJSON(endpoint string, target any) error {
	_, body, err := c.Request(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(body), target); err != nil {
		return fmt.Errorf("decode %s: %w", endpoint, err)
	}
	return nil
}

func (c Client) ExecuteTool(name string, input map[string]any, target any) error {
	payload, err := json.Marshal(tools.ExecuteRequest{Name: name, Input: input})
	if err != nil {
		return err
	}
	_, body, err := c.Request(http.MethodPost, c.ServerURL+"/tools/execute", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	var envelope ToolEnvelope
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		return fmt.Errorf("decode tool %s response: %w", name, err)
	}
	if target == nil || len(envelope.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, target); err != nil {
		return fmt.Errorf("decode tool %s result: %w", name, err)
	}
	return nil
}

func (c Client) Request(method string, endpoint string, body io.Reader) (int, string, error) {
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultRequestTimeout}
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return 0, "", err
	}
	if body != nil {
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}
	addAuthHeaders(req)
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	out := strings.TrimSpace(string(data))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, out, fmt.Errorf("%s %s failed: %s: %s", method, endpoint, resp.Status, out)
	}
	return resp.StatusCode, out, nil
}

func addAuthHeaders(req *http.Request) {
	if req == nil {
		return
	}
	if token := os.Getenv("REVIEW_API_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-7review-Token", token)
	}
}
