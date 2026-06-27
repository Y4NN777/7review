package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Y4NN777/7review/agent/app"
	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/tools"
	"github.com/Y4NN777/7review/agent/ui"
)

const operatorRequestTimeout = 60 * time.Second
const operatorStreamTimeout = 10 * time.Minute
const maxSSEEventBytes = 4 << 20

func main() {
	if len(os.Args) > 1 && os.Args[1] == "status" {
		if err := runStatus(os.Args[2:], os.Stdout); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		runSetup()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "chat" {
		runChat()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "tui" {
		if err := runTUI(os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "runs" {
		runListRuns()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "run" {
		runGetRun()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "approve" {
		runApprove()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "publish-final" {
		runPublishFinal()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := runHealthcheck(os.Args[2:], operatorRequestHTTPClient()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	server, err := app.NewServer()
	if err != nil {
		log.Fatal(err)
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func runListRuns() {
	serverURL := commandServerURL(os.Args[2:])
	body, err := requestAgent(operatorRequestHTTPClient(), http.MethodGet, strings.TrimRight(serverURL, "/")+"/runs", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(body)
}

func runGetRun() {
	serverURL := "http://localhost:8080"
	runID := ""
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch flagName(arg) {
		case "--server":
			serverURL = flagValue(args, &i)
		case "--run":
			runID = flagValue(args, &i)
		default:
			if !strings.HasPrefix(arg, "-") && runID == "" {
				runID = arg
			}
		}
	}
	if runID == "" {
		fmt.Fprintln(os.Stderr, "missing run id")
		os.Exit(1)
	}
	endpoint := strings.TrimRight(serverURL, "/") + "/run?id=" + url.QueryEscape(runID)
	body, err := requestAgent(operatorRequestHTTPClient(), http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(body)
}

func runApprove() {
	opts, err := parseApprovalArgs(os.Args[2:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	endpoint := strings.TrimRight(opts.serverURL, "/") + "/approve?"
	if opts.runID != "" {
		endpoint += "run=" + url.QueryEscape(opts.runID)
	} else {
		endpoint += "project=" + url.QueryEscape(opts.projectID) + "&mr=" + url.QueryEscape(opts.mrIID)
	}
	if _, err := requestAgent(operatorRequestHTTPClient(), http.MethodPost, endpoint, strings.NewReader(opts.report)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("approval queued for %s\n", opts.approvalTarget())
}

func runPublishFinal() {
	opts, err := parsePublishArgs(os.Args[2:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	endpoint := strings.TrimRight(opts.serverURL, "/") + "/publish/final?run=" + url.QueryEscape(opts.runID)
	if _, err := requestAgent(operatorRequestHTTPClient(), http.MethodPost, endpoint, strings.NewReader(opts.report)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("final publish queued for %s\n", opts.runID)
}

func runSetup() {
	opts := ui.SetupOptions{OutputPath: ".env"}
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--force":
			opts.Force = true
		case "--plain":
			opts.Plain = true
		default:
			opts.OutputPath = arg
		}
	}
	if err := ui.RunSetupWizard(os.Stdin, os.Stdout, opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runChat() {
	opts := ui.ChatOptions{}
	serverURL := "http://localhost:8080"
	runID := ""
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch flagName(arg) {
		case "--plain":
			opts.Plain = true
		case "--server":
			serverURL = flagValue(args, &i)
		case "--run":
			runID = flagValue(args, &i)
		}
	}
	chatCtx := ui.ChatContext{}
	var responder ui.ChatResponder
	if runID != "" {
		chatCtx.ConfigLoaded = true
		chatCtx.RunID = runID
		responder = &remoteRunChatResponder{
			serverURL:  strings.TrimRight(serverURL, "/"),
			runID:      runID,
			httpClient: operatorStreamHTTPClient(),
		}
	} else {
		cfg, err := config.LoadConfig()
		if err != nil {
			chatCtx.ConfigError = err.Error()
			responder = ui.StaticResponder{Message: "Config is not ready. Run 7review setup, then 7review status. Missing config: " + err.Error()}
		} else {
			chatCtx.ConfigLoaded = true
			chatCtx.HeadroomURL = cfg.HeadroomURL
			chatCtx.MemPalaceURL = cfg.MemPalaceURL
			modelOrchestrator, err := orchestrator.BuildOrchestrator(cfg)
			if err != nil {
				chatCtx.ConfigLoaded = false
				chatCtx.ConfigError = err.Error()
				responder = ui.StaticResponder{Message: "Model orchestration is not ready: " + err.Error()}
			} else {
				responder = &modelChatResponder{
					orchestrator: modelOrchestrator,
					cfg:          cfg,
				}
			}
		}
	}
	if err := ui.RunChat(context.Background(), os.Stdin, os.Stdout, chatCtx, responder, opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type remoteRunChatResponder struct {
	serverURL  string
	runID      string
	httpClient *http.Client
}

func (r *remoteRunChatResponder) StreamRespond(ctx context.Context, input string, emit func(string) error) error {
	if r.httpClient == nil {
		r.httpClient = operatorStreamHTTPClient()
	}
	payload, _ := json.Marshal(map[string]string{"message": input})
	endpoint := r.serverURL + "/chat/stream?run=" + url.QueryEscape(r.runID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	addAgentAuthHeaders(req)
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("server chat failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return readSSE(resp.Body, emit)
}

func readSSE(body io.Reader, emit func(string) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEEventBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "{}" {
			continue
		}
		var event struct {
			Delta string `json:"delta"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		if event.Error != "" {
			return errors.New(event.Error)
		}
		if event.Delta != "" {
			if err := emit(event.Delta); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read SSE stream: %w", err)
	}
	return nil
}

type approvalCommandOptions struct {
	serverURL string
	runID     string
	projectID string
	mrIID     string
	report    string
}

func (o approvalCommandOptions) approvalTarget() string {
	if o.runID != "" {
		return o.runID
	}
	return o.projectID + "!" + o.mrIID
}

type publishCommandOptions struct {
	serverURL string
	runID     string
	report    string
}

func parseApprovalArgs(args []string) (approvalCommandOptions, error) {
	opts := approvalCommandOptions{serverURL: "http://localhost:8080"}
	var reportFile string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case flagName(arg) == "--server":
			opts.serverURL = flagValue(args, &i)
		case flagName(arg) == "--run":
			opts.runID = flagValue(args, &i)
		case flagName(arg) == "--project":
			opts.projectID = flagValue(args, &i)
		case flagName(arg) == "--mr":
			opts.mrIID = flagValue(args, &i)
		case flagName(arg) == "--report-file":
			reportFile = flagValue(args, &i)
		case flagName(arg) == "--report":
			opts.report = flagValue(args, &i)
		case !strings.HasPrefix(arg, "-") && opts.runID == "":
			opts.runID = arg
		}
	}
	if opts.runID == "" && (opts.projectID == "" || opts.mrIID == "") {
		return opts, errors.New("approve requires --run=<id> or --project=<id> and --mr=<iid>")
	}
	if reportFile != "" {
		report, err := os.ReadFile(reportFile)
		if err != nil {
			return opts, fmt.Errorf("read report file: %w", err)
		}
		opts.report = string(report)
	}
	return opts, nil
}

func parsePublishArgs(args []string) (publishCommandOptions, error) {
	opts := publishCommandOptions{serverURL: "http://localhost:8080"}
	var reportFile string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case flagName(arg) == "--server":
			opts.serverURL = flagValue(args, &i)
		case flagName(arg) == "--run":
			opts.runID = flagValue(args, &i)
		case flagName(arg) == "--report-file":
			reportFile = flagValue(args, &i)
		case flagName(arg) == "--report":
			opts.report = flagValue(args, &i)
		case !strings.HasPrefix(arg, "-") && opts.runID == "":
			opts.runID = arg
		}
	}
	if opts.runID == "" {
		return opts, errors.New("publish-final requires --run=<id> or a positional run id")
	}
	if reportFile != "" {
		report, err := os.ReadFile(reportFile)
		if err != nil {
			return opts, fmt.Errorf("read report file: %w", err)
		}
		opts.report = string(report)
	}
	return opts, nil
}

func commandServerURL(args []string) string {
	for i := 0; i < len(args); i++ {
		if flagName(args[i]) == "--server" {
			return flagValue(args, &i)
		}
	}
	return "http://localhost:8080"
}

func runHealthcheck(args []string, client *http.Client) error {
	endpoint := "http://127.0.0.1:8080/health"
	for i := 0; i < len(args); i++ {
		switch flagName(args[i]) {
		case "--url":
			endpoint = flagValue(args, &i)
		case "--server":
			endpoint = strings.TrimRight(flagValue(args, &i), "/") + "/health"
		}
	}
	_, err := requestAgent(client, http.MethodGet, endpoint, nil)
	return err
}

func flagName(arg string) string {
	if name, _, ok := strings.Cut(arg, "="); ok {
		return name
	}
	return arg
}

func flagValue(args []string, idx *int) string {
	arg := args[*idx]
	if _, value, ok := strings.Cut(arg, "="); ok {
		return value
	}
	if *idx+1 >= len(args) {
		return ""
	}
	(*idx)++
	return args[*idx]
}

func requestAgent(client *http.Client, method string, endpoint string, body io.Reader) (string, error) {
	_, out, err := requestAgentRaw(client, method, endpoint, body)
	return out, err
}

func requestAgentRaw(client *http.Client, method string, endpoint string, body io.Reader) (int, string, error) {
	if client == nil {
		client = operatorRequestHTTPClient()
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return 0, "", err
	}
	if body != nil {
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}
	addAgentAuthHeaders(req)
	resp, err := client.Do(req)
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

func operatorRequestHTTPClient() *http.Client {
	return &http.Client{Timeout: operatorRequestTimeout}
}

func operatorStreamHTTPClient() *http.Client {
	return &http.Client{Timeout: operatorStreamTimeout}
}

func addAgentAuthHeaders(req *http.Request) {
	if req == nil {
		return
	}
	if token := os.Getenv("REVIEW_API_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-7review-Token", token)
	}
}

type modelChatResponder struct {
	orchestrator *orchestrator.Orchestrator
	cfg          *config.Config
	history      []string
}

func (r *modelChatResponder) StreamRespond(ctx context.Context, input string, emit func(string) error) error {
	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	userMessage := r.userMessage(input)
	rc := review.NewContext(review.Request{Provider: "cli", ProjectID: "local", ChangeID: "chat"})
	out, err := r.orchestrator.StreamComplete(callCtx, rc, orchestrator.RoleFormatter, "chat", chatSystemPrompt(r.cfg), userMessage, emit)
	if err != nil {
		return err
	}
	r.history = append(r.history, "user: "+input, "agent: "+out)
	if len(r.history) > 12 {
		r.history = r.history[len(r.history)-12:]
	}
	return nil
}

func (r *modelChatResponder) userMessage(input string) string {
	var b strings.Builder
	if len(r.history) > 0 {
		b.WriteString("Recent chat history:\n")
		for _, item := range r.history {
			b.WriteString(item)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("User message:\n")
	b.WriteString(input)
	return b.String()
}

func chatSystemPrompt(cfg *config.Config) string {
	instructions := readInstructions(cfg.InstructionsPath)
	toolNames := make([]string, 0, len(tools.Catalog()))
	for _, tool := range tools.Catalog() {
		toolNames = append(toolNames, "- "+tool.Name+": "+tool.Description)
	}
	return strings.Join([]string{
		"You are 7review's live engineering copilot, embedded in a production code-review agent.",
		"Always-on instructions:",
		instructions,
		"Your job is to help engineers operate and iterate with the agent while preserving the review lifecycle: webhook -> SCM enrichment -> diff/context -> model review -> validation -> draft -> HIL approval -> final publish -> memory write.",
		"Communication rules:",
		"- Be concise, direct, and operational.",
		"- Always separate known state from assumptions.",
		"- Never invent runtime state, review results, dependency health, or SCM data.",
		"- If state is unknown, give the exact command or endpoint that verifies it.",
		"- Prefer one clear next command over broad advice.",
		"- When the engineer asks what to do next, answer with the next command and why it matters.",
		"- If the engineer asks about a finding, explain risk, evidence, and what would prove or disprove it.",
		"- Do not claim final approval, publish final reports, or write memory unless HIL approval is explicitly present.",
		"- Treat Headroom and MemPalace as required dependencies, not optional helpers.",
		"- Operator commands and curl examples need REVIEW_API_TOKEN in the environment or an Authorization bearer token.",
		"- Keep standard command names literal: setup, status, chat, docker compose, /ready, /runs, /run, /chat/stream.",
		"Available local commands:",
		"- 7review setup",
		"- 7review status",
		"- 7review status --server <agent-url>",
		"- 7review runs --server <agent-url>",
		"- 7review run <run-id> --server <agent-url>",
		"- 7review chat",
		"- 7review chat --run <run-id> --server <agent-url>",
		"- 7review approve --run <run-id> --report-file <path> --server <agent-url>",
		"- 7review approve --project <project-id> --mr <iid> --report-file <path> --server <agent-url>",
		"- 7review publish-final --run <run-id> --report-file <path> --server <agent-url>",
		"- docker compose up --build",
		"- curl <agent-url>/ready",
		"Model-facing tools:",
		strings.Join(toolNames, "\n"),
		"Configured endpoints:",
		"Headroom: " + cfg.HeadroomURL,
		"MemPalace: " + cfg.MemPalaceURL,
		"GitLab URL: " + cfg.GitLabURL,
		"GitHub API URL: " + cfg.GitHubAPIURL,
	}, "\n")
}

func readInstructions(path string) string {
	if path == "" {
		path = "./agent/instructions.md"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "agent instructions unavailable: " + err.Error()
	}
	return string(data)
}

type statusCommandOptions struct {
	serverURL string
	remote    bool
	plain     bool
}

type remoteReadiness struct {
	Ready        bool              `json:"ready"`
	Dependencies map[string]string `json:"dependencies"`
	Queue        remoteQueueStatus `json:"queue,omitempty"`
}

type remoteQueueStatus struct {
	Depth     int    `json:"depth"`
	Capacity  int    `json:"capacity"`
	Available int    `json:"available"`
	Enqueued  uint64 `json:"enqueued"`
	Completed uint64 `json:"completed"`
	Failed    uint64 `json:"failed"`
}

func runStatus(args []string, out io.Writer) error {
	opts := parseStatusArgs(args)
	if opts.remote {
		view, ready, err := remoteStatusView(operatorRequestHTTPClient(), opts)
		fmt.Fprintln(out, ui.RenderStatus(view))
		if err != nil {
			return err
		}
		if !ready {
			return errors.New("agent is not ready")
		}
		return nil
	}
	view, ready := localStatusView(opts.plain)
	fmt.Fprintln(out, ui.RenderStatus(view))
	if !ready {
		return errors.New("local configuration is not ready")
	}
	return nil
}

func parseStatusArgs(args []string) statusCommandOptions {
	opts := statusCommandOptions{serverURL: "http://localhost:8080"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch flagName(arg) {
		case "--server":
			opts.serverURL = flagValue(args, &i)
			opts.remote = true
		case "--plain":
			opts.plain = true
		}
	}
	return opts
}

func localStatusView(plain bool) (ui.StatusView, bool) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return ui.StatusView{
			Title: "7review status",
			Plain: plain,
			Dependencies: []ui.DependencyStatus{
				{Name: "config", Ready: false, Detail: err.Error()},
			},
		}, false
	}
	return ui.StatusView{
		Title: "7review status",
		Plain: plain,
		Dependencies: []ui.DependencyStatus{
			{Name: "headroom", URL: cfg.HeadroomURL, Ready: cfg.HeadroomURL != ""},
			{Name: "mempalace", URL: cfg.MemPalaceURL, Ready: cfg.MemPalaceURL != ""},
		},
	}, true
}

func remoteStatusView(client *http.Client, opts statusCommandOptions) (ui.StatusView, bool, error) {
	serverURL := strings.TrimRight(opts.serverURL, "/")
	endpoint := serverURL + "/ready"
	statusCode, body, err := requestAgentRaw(client, http.MethodGet, endpoint, nil)
	if err != nil && body == "" {
		return ui.StatusView{
			Title: "7review status " + serverURL,
			Plain: opts.plain,
			Dependencies: []ui.DependencyStatus{{
				Name:   "agent",
				URL:    endpoint,
				Ready:  false,
				Detail: err.Error(),
			}},
		}, false, err
	}
	var ready remoteReadiness
	if decodeErr := json.Unmarshal([]byte(body), &ready); decodeErr != nil {
		return ui.StatusView{
			Title: "7review status " + serverURL,
			Plain: opts.plain,
			Dependencies: []ui.DependencyStatus{{
				Name:   "agent",
				URL:    endpoint,
				Ready:  false,
				Detail: "invalid readiness response: " + decodeErr.Error(),
			}},
		}, false, decodeErr
	}
	view := ui.StatusView{
		Title: "7review status " + serverURL,
		Plain: opts.plain,
		Dependencies: []ui.DependencyStatus{{
			Name:   "agent",
			URL:    endpoint,
			Ready:  ready.Ready && err == nil,
			Detail: fmt.Sprintf("http=%d", statusCode),
		}},
	}
	for name, detail := range ready.Dependencies {
		view.Dependencies = append(view.Dependencies, ui.DependencyStatus{
			Name:   name,
			Ready:  dependencyReady(detail),
			Detail: remoteDependencyDetail(name, detail, ready.Queue),
		})
	}
	return view, ready.Ready && err == nil, err
}

func dependencyReady(detail string) bool {
	detail = strings.TrimSpace(strings.ToLower(detail))
	return detail == "ok" || strings.HasPrefix(detail, "ok ")
}

func remoteDependencyDetail(name, detail string, queue remoteQueueStatus) string {
	if name != "queue" || queue.Capacity == 0 {
		return detail
	}
	return fmt.Sprintf("%s available=%d enqueued=%d completed=%d failed=%d", detail, queue.Available, queue.Enqueued, queue.Completed, queue.Failed)
}
