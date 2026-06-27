package app

import (
	"fmt"
	"io"
)

const webhookMaxBodyBytes int64 = 2 << 20

func readWebhookBody(body io.Reader) ([]byte, error) {
	return readBoundedBody(body, webhookMaxBodyBytes)
}

func readBoundedBody(body io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("request body exceeds %d bytes", maxBytes)
	}
	return data, nil
}
