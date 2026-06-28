package main

import (
	"net/http"

	"github.com/Y4NN777/7review/cmd/7review/operator"
)

func getJSON(client *http.Client, endpoint string, target any) error {
	return operator.NewClient("", client).GetJSON(endpoint, target)
}

func executeRemoteTool(client *http.Client, serverURL string, name string, input map[string]any, target any) error {
	return operator.NewClient(serverURL, client).ExecuteTool(name, input, target)
}
