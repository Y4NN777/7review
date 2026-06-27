package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type remoteRunEvent struct {
	At      time.Time         `json:"at"`
	Type    string            `json:"type"`
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Meta    map[string]string `json:"meta"`
}

type remoteRunDetail struct {
	ID         string           `json:"id"`
	Status     string           `json:"status"`
	Title      string           `json:"title"`
	EventCount int              `json:"event_count"`
	Events     []remoteRunEvent `json:"events"`
}

func runHistory() {
	opts := parseHistoryArgs(os.Args[2:])
	if opts.runID == "" {
		fmt.Fprintln(os.Stderr, "missing run id")
		os.Exit(1)
	}
	endpoint := strings.TrimRight(opts.serverURL, "/") + "/run?id=" + url.QueryEscape(opts.runID)
	body, err := requestAgent(operatorRequestHTTPClient(), http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var detail remoteRunDetail
	if err := json.Unmarshal([]byte(body), &detail); err != nil {
		fmt.Fprintln(os.Stderr, "decode run history:", err)
		os.Exit(1)
	}
	fmt.Println(renderRunHistory(detail))
}

type historyCommandOptions struct {
	serverURL string
	runID     string
}

func parseHistoryArgs(args []string) historyCommandOptions {
	opts := historyCommandOptions{serverURL: "http://localhost:8080"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch flagName(arg) {
		case "--server":
			opts.serverURL = flagValue(args, &i)
		case "--run":
			opts.runID = flagValue(args, &i)
		default:
			if !strings.HasPrefix(arg, "-") && opts.runID == "" {
				opts.runID = arg
			}
		}
	}
	return opts
}

func renderRunHistory(run remoteRunDetail) string {
	title := strings.TrimSpace(run.Title)
	if title == "" {
		title = run.ID
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("%s  %s", run.ID, run.Status))
	lines = append(lines, title)
	lines = append(lines, fmt.Sprintf("history %d events", len(run.Events)))
	for _, event := range run.Events {
		lines = append(lines, renderRunEvent(event))
	}
	return strings.Join(lines, "\n")
}

func renderRunEvent(event remoteRunEvent) string {
	at := "-"
	if !event.At.IsZero() {
		at = event.At.UTC().Format(time.RFC3339)
	}
	parts := []string{at, strings.TrimSpace(event.Type)}
	if strings.TrimSpace(event.Status) != "" {
		parts = append(parts, strings.TrimSpace(event.Status))
	}
	if strings.TrimSpace(event.Message) != "" {
		parts = append(parts, strings.TrimSpace(event.Message))
	}
	line := strings.Join(nonEmptyHistoryParts(parts), "  ")
	if len(event.Meta) == 0 {
		return line
	}
	var keys []string
	for key := range event.Meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var meta []string
	for _, key := range keys {
		value := strings.TrimSpace(event.Meta[key])
		if strings.TrimSpace(key) != "" && value != "" {
			meta = append(meta, key+"="+value)
		}
	}
	if len(meta) == 0 {
		return line
	}
	return line + "  " + strings.Join(meta, " ")
}

func nonEmptyHistoryParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
