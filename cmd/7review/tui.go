package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Y4NN777/7review/agent/tools"
	"github.com/Y4NN777/7review/agent/ui"
)

type tuiCommandOptions struct {
	serverURL      string
	runID          string
	plain          bool
	watch          bool
	refreshEvery   time.Duration
	clearOnRefresh bool
}

type remoteRunRow struct {
	ID          string    `json:"id"`
	Provider    string    `json:"provider"`
	ProjectID   string    `json:"project_id"`
	ChangeID    string    `json:"change_id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	Error       string    `json:"error"`
	WebURL      string    `json:"web_url"`
	UpdatedAt   time.Time `json:"updated_at"`
	EventCount  int       `json:"event_count"`
	Events      []any     `json:"events"`
	Findings    []any     `json:"findings"`
	DraftReport string    `json:"draft_report"`
	FinalReport string    `json:"final_report"`
	HILApproved bool      `json:"hil_approved"`
}

type remoteToolEnvelope struct {
	Name   string          `json:"name"`
	Result json.RawMessage `json:"result"`
}

type remoteProviderStatus struct {
	Mode               string                    `json:"mode"`
	ActiveProvider     string                    `json:"active_provider"`
	OrchestratorConfig string                    `json:"orchestrator_config"`
	Providers          []remoteProviderStatusRow `json:"providers"`
	Roles              []remoteRoleStatus        `json:"roles"`
}

type remoteProviderStatusRow struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	BaseURL    string `json:"base_url"`
	Reason     string `json:"reason"`
}

type remoteRoleStatus struct {
	Role        string   `json:"role"`
	Primary     string   `json:"primary"`
	Fallbacks   []string `json:"fallbacks"`
	MaxTokens   int      `json:"max_tokens"`
	Parallel    bool     `json:"parallel"`
	MaxParallel int      `json:"max_parallel"`
}

type remoteSkillStatus struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Loaded bool   `json:"loaded"`
}

type remoteConfigStatus struct {
	ListenAddr       string `json:"listen_addr"`
	CorpusRoot       string `json:"corpus_root"`
	MemoryDir        string `json:"memory_dir"`
	HILChannel       string `json:"hil_channel"`
	Provider         string `json:"provider"`
	ReviewModel      string `json:"review_model"`
	SmallModel       string `json:"small_model"`
	Orchestrator     string `json:"orchestrator_config"`
	HasGitLab        bool   `json:"has_gitlab"`
	HasGitHub        bool   `json:"has_github"`
	HasOpenAI        bool   `json:"has_openai"`
	HasOpenRouter    bool   `json:"has_openrouter"`
	HasDeepSeek      bool   `json:"has_deepseek"`
	HasAnthropic     bool   `json:"has_anthropic"`
	HasMistral       bool   `json:"has_mistral"`
	HasGemini        bool   `json:"has_gemini"`
	HasOllama        bool   `json:"has_ollama"`
	HeadroomURL      string `json:"headroom_url"`
	MemPalaceURL     string `json:"mempalace_url"`
	WebhookWorkers   int    `json:"webhook_workers"`
	WebhookQueueSize int    `json:"webhook_queue_size"`
}

func runTUI(args []string, out io.Writer) error {
	opts := parseTUIArgs(args)
	client := operatorRequestHTTPClient()
	if !opts.watch {
		view, err := remoteConsoleView(client, opts)
		fmt.Fprintln(out, ui.RenderConsole(view))
		return err
	}
	for {
		view, _ := remoteConsoleView(client, opts)
		if opts.clearOnRefresh {
			fmt.Fprint(out, "\x1b[2J\x1b[H")
		}
		fmt.Fprintln(out, ui.RenderConsole(view))
		time.Sleep(opts.refreshEvery)
	}
}

func parseTUIArgs(args []string) tuiCommandOptions {
	opts := tuiCommandOptions{
		serverURL:      "http://localhost:8080",
		refreshEvery:   2 * time.Second,
		clearOnRefresh: true,
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch flagName(arg) {
		case "--server":
			opts.serverURL = flagValue(args, &i)
		case "--run":
			opts.runID = flagValue(args, &i)
		case "--plain":
			opts.plain = true
			opts.clearOnRefresh = false
		case "--watch":
			opts.watch = true
		case "--refresh", "--interval":
			opts.refreshEvery = parseRefreshInterval(flagValue(args, &i), opts.refreshEvery)
		case "--no-clear":
			opts.clearOnRefresh = false
		default:
			if !strings.HasPrefix(arg, "-") && opts.runID == "" {
				opts.runID = arg
			}
		}
	}
	return opts
}

func remoteConsoleView(client *http.Client, opts tuiCommandOptions) (ui.ConsoleView, error) {
	serverURL := strings.TrimRight(opts.serverURL, "/")
	view := ui.ConsoleView{
		Server:       serverURL,
		Plain:        opts.plain,
		Watch:        opts.watch,
		RefreshedAt:  time.Now().UTC(),
		RefreshEvery: opts.refreshEvery,
	}
	var failures []string

	readyView, ready, err := remoteStatusView(client, statusCommandOptions{serverURL: serverURL, remote: true, plain: true})
	view.Ready = ready
	view.Dependencies = readyView.Dependencies
	for _, dep := range readyView.Dependencies {
		if dep.Name == "queue" {
			view.Queue = parseQueueFromDetail(dep.Detail)
			break
		}
	}
	if err != nil {
		failures = append(failures, err.Error())
	}

	var toolsOut []ui.ToolRow
	if err := getJSON(client, serverURL+"/tools", &toolsOut); err != nil {
		failures = append(failures, err.Error())
	} else {
		view.Tools = toolsOut
	}

	var runs []remoteRunRow
	if err := executeRemoteTool(client, serverURL, "list_runs", nil, &runs); err != nil {
		failures = append(failures, err.Error())
	} else {
		view.Runs = toUIRunRows(runs)
	}

	var providers remoteProviderStatus
	if err := executeRemoteTool(client, serverURL, "list_provider_status", nil, &providers); err != nil {
		failures = append(failures, err.Error())
	} else {
		view.Providers = toUIProviderRows(providers.Providers)
		view.Roles = toUIRoleRows(providers.Roles)
	}

	var skills []remoteSkillStatus
	if err := executeRemoteTool(client, serverURL, "list_skills", nil, &skills); err != nil {
		failures = append(failures, err.Error())
	} else {
		view.Skills = toUISkillRows(skills)
	}

	runID := opts.runID
	if runID == "" && len(view.Runs) > 0 {
		runID = view.Runs[0].ID
	}
	if runID != "" {
		var detail remoteRunRow
		if err := executeRemoteTool(client, serverURL, "get_run", map[string]any{"id": runID}, &detail); err != nil {
			failures = append(failures, err.Error())
		} else {
			view.ActiveRun = toUIRunDetail(detail)
		}
	}

	view.Warnings = failures
	if len(failures) > 0 {
		return view, errors.New(strings.Join(failures, "; "))
	}
	return view, nil
}

func parseRefreshInterval(value string, fallback time.Duration) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	interval, err := time.ParseDuration(value)
	if err != nil {
		interval, err = time.ParseDuration(value + "s")
	}
	if err != nil || interval <= 0 {
		return fallback
	}
	return interval
}

func getJSON(client *http.Client, endpoint string, target any) error {
	_, body, err := requestAgentRaw(client, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(body), target); err != nil {
		return fmt.Errorf("decode %s: %w", endpoint, err)
	}
	return nil
}

func executeRemoteTool(client *http.Client, serverURL string, name string, input map[string]any, target any) error {
	payload, err := json.Marshal(tools.ExecuteRequest{Name: name, Input: input})
	if err != nil {
		return err
	}
	_, body, err := requestAgentRaw(client, http.MethodPost, strings.TrimRight(serverURL, "/")+"/tools/execute", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	var envelope remoteToolEnvelope
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

func toUIRunRows(runs []remoteRunRow) []ui.RunRow {
	out := make([]ui.RunRow, 0, len(runs))
	for _, run := range runs {
		out = append(out, ui.RunRow{
			ID:          run.ID,
			Provider:    run.Provider,
			ProjectID:   run.ProjectID,
			ChangeID:    run.ChangeID,
			Title:       run.Title,
			Status:      run.Status,
			Error:       run.Error,
			WebURL:      run.WebURL,
			UpdatedAt:   run.UpdatedAt,
			EventCount:  run.EventCount,
			HILApproved: run.HILApproved,
		})
	}
	return out
}

func toUIRunDetail(run remoteRunRow) *ui.RunDetail {
	return &ui.RunDetail{
		RunRow: ui.RunRow{
			ID:          run.ID,
			Provider:    run.Provider,
			ProjectID:   run.ProjectID,
			ChangeID:    run.ChangeID,
			Title:       run.Title,
			Status:      run.Status,
			Error:       run.Error,
			WebURL:      run.WebURL,
			UpdatedAt:   run.UpdatedAt,
			HILApproved: run.HILApproved,
		},
		Findings:    len(run.Findings),
		DraftBytes:  len(run.DraftReport),
		FinalBytes:  len(run.FinalReport),
		ReportReady: strings.TrimSpace(run.DraftReport) != "" || strings.TrimSpace(run.FinalReport) != "",
		EventCount:  run.EventCount,
		LatestEvent: latestRemoteRunEvent(run.Events),
	}
}

func latestRemoteRunEvent(events []any) string {
	if len(events) == 0 {
		return ""
	}
	raw, ok := events[len(events)-1].(map[string]any)
	if !ok {
		return ""
	}
	eventType, _ := raw["type"].(string)
	status, _ := raw["status"].(string)
	message, _ := raw["message"].(string)
	parts := []string{strings.TrimSpace(eventType)}
	if strings.TrimSpace(status) != "" {
		parts = append(parts, strings.TrimSpace(status))
	}
	if strings.TrimSpace(message) != "" {
		parts = append(parts, strings.TrimSpace(message))
	}
	return strings.Join(nonEmptyStrings(parts), " ")
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func toUIProviderRows(rows []remoteProviderStatusRow) []ui.ProviderRow {
	out := make([]ui.ProviderRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ui.ProviderRow{
			Name:       row.Name,
			Configured: row.Configured,
			BaseURL:    row.BaseURL,
			Reason:     row.Reason,
		})
	}
	return out
}

func toUIRoleRows(rows []remoteRoleStatus) []ui.RoleRow {
	out := make([]ui.RoleRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ui.RoleRow{
			Role:        row.Role,
			Primary:     row.Primary,
			Fallbacks:   append([]string(nil), row.Fallbacks...),
			MaxTokens:   row.MaxTokens,
			Parallel:    row.Parallel,
			MaxParallel: row.MaxParallel,
		})
	}
	return out
}

func toUISkillRows(rows []remoteSkillStatus) []ui.SkillRow {
	out := make([]ui.SkillRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ui.SkillRow{Name: row.Name, Loaded: row.Loaded, Path: row.Path})
	}
	return out
}

func parseQueueFromDetail(detail string) ui.QueueView {
	var q ui.QueueView
	for _, field := range strings.Fields(detail) {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch key {
		case "available":
			fmt.Sscanf(value, "%d", &q.Available)
		case "enqueued":
			fmt.Sscanf(value, "%d", &q.Enqueued)
		case "completed":
			fmt.Sscanf(value, "%d", &q.Completed)
		case "failed":
			fmt.Sscanf(value, "%d", &q.Failed)
		}
	}
	if _, rest, ok := strings.Cut(detail, "depth="); ok {
		fmt.Sscanf(rest, "%d capacity=%d", &q.Depth, &q.Capacity)
	}
	return q
}
