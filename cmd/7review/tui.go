package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Y4NN777/7review/agent/tools"
	"github.com/Y4NN777/7review/agent/ui"
	tea "github.com/charmbracelet/bubbletea"
)

type tuiCommandOptions struct {
	serverURL      string
	runID          string
	plain          bool
	watch          bool
	once           bool
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
	EmbeddingModel   string `json:"embedding_model"`
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

type remoteMemoryProposalStatus struct {
	Run        string               `json:"run"`
	Approved   bool                 `json:"approved"`
	Proposal   remoteUpdateProposal `json:"proposal"`
	FinalBytes int                  `json:"final_bytes"`
}

type remoteDiffSummary struct {
	Run          string              `json:"run"`
	FileCount    int                 `json:"file_count"`
	TotalTokens  int                 `json:"total_tokens"`
	Additions    int                 `json:"additions"`
	Deletions    int                 `json:"deletions"`
	Files        []remoteFileDiff    `json:"files"`
	ChangedFiles []remoteChangedFile `json:"changed_files"`
}

type remoteFileDiff struct {
	Path       string `json:"path"`
	TokenCount int    `json:"token_count"`
	PatchLines int    `json:"patch_lines"`
}

type remoteChangedFile struct {
	Path      string `json:"path"`
	OldPath   string `json:"old_path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	HasPatch  bool   `json:"has_patch"`
}

type remoteUpdateProposal struct {
	Conventions []string       `json:"Conventions"`
	Decisions   []string       `json:"Decisions"`
	Vectors     []remoteVector `json:"Vectors"`
}

type remoteVector struct {
	ID   string `json:"ID"`
	Text string `json:"Text"`
}

func runTUI(args []string, out io.Writer) error {
	opts := parseTUIArgs(args)
	client := operatorRequestHTTPClient()
	if opts.once {
		view, err := remoteConsoleView(client, opts)
		fmt.Fprintln(out, ui.RenderConsole(view))
		return err
	}
	program := tea.NewProgram(newConsoleTUIModel(client, opts), tea.WithOutput(out), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

type consoleTUIModel struct {
	client           *http.Client
	opts             tuiCommandOptions
	view             ui.ConsoleView
	err              error
	loading          bool
	help             bool
	message          string
	input            string
	commandRunning   bool
	transcript       []ui.ConsoleTranscriptItem
	stream           <-chan consoleStreamMsg
	cancel           context.CancelFunc
	transcriptScroll int
	paletteOpen      bool
	paletteSelected  int
	width            int
	height           int
}

type SlashCommand struct {
	Name        string
	Aliases     []string
	Usage       string
	Description string
	RequiresRun bool
	Examples    []string
}

type slashCommandMatch struct {
	Command SlashCommand
	Indices []int
}

var slashCommands = []SlashCommand{
	{Name: "/help", Aliases: []string{"?", "/?"}, Usage: "/help", Description: "Show slash commands and examples."},
	{Name: "/status", Aliases: []string{"/ready"}, Usage: "/status", Description: "Show agent readiness and sidecar status."},
	{Name: "/config", Aliases: []string{"/env"}, Usage: "/config", Description: "Show redacted runtime configuration."},
	{Name: "/providers", Aliases: []string{"/models"}, Usage: "/providers", Description: "Show model providers and role routes."},
	{Name: "/skills", Aliases: []string{"/skill"}, Usage: "/skills", Description: "Show loaded review skills."},
	{Name: "/tools", Aliases: []string{"/tool"}, Usage: "/tools", Description: "Show implemented operator tools."},
	{Name: "/sessions", Aliases: []string{"/runs"}, Usage: "/sessions [status] [limit]", Description: "List review sessions.", Examples: []string{"/sessions drafted 5"}},
	{Name: "/run", Aliases: []string{"/current"}, Usage: "/run", Description: "Show current run summary.", RequiresRun: true},
	{Name: "/history", Aliases: []string{"/events"}, Usage: "/history [type] [limit]", Description: "Show current run timeline.", RequiresRun: true, Examples: []string{"/history chat_message 20"}},
	{Name: "/diff", Aliases: []string{"/changes"}, Usage: "/diff", Description: "Show changed files and patch summary.", RequiresRun: true},
	{Name: "/draft", Aliases: []string{"/report"}, Usage: "/draft [output-file]", Description: "Show or write the current draft report.", RequiresRun: true, Examples: []string{"/draft final.md"}},
	{Name: "/memory", Aliases: []string{"/mempalace"}, Usage: "/memory", Description: "Preview approved MemPalace proposal.", RequiresRun: true},
	{Name: "/approve", Aliases: []string{"/hil"}, Usage: "/approve --report-file <path>", Description: "Approve and publish the final review.", RequiresRun: true, Examples: []string{"/approve --report-file final.md"}},
	{Name: "/publish-final", Aliases: []string{"/publish"}, Usage: "/publish-final --report-file <path>", Description: "Retry final report publishing.", RequiresRun: true, Examples: []string{"/publish-final --report-file final.md"}},
}

type consoleViewMsg struct {
	view ui.ConsoleView
	err  error
}

type consoleTickMsg time.Time

type consoleCommandMsg struct {
	input string
	out   string
	err   error
}

type consoleStreamMsg struct {
	delta string
	err   error
	done  bool
}

func newConsoleTUIModel(client *http.Client, opts tuiCommandOptions) consoleTUIModel {
	if opts.refreshEvery <= 0 {
		opts.refreshEvery = 2 * time.Second
	}
	opts.watch = true
	opts.once = false
	return consoleTUIModel{client: client, opts: opts, loading: true, message: "loading"}
}

func (m consoleTUIModel) Init() tea.Cmd {
	return fetchConsoleViewCmd(m.client, m.opts)
}

func (m consoleTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.paletteOpen {
				m.paletteOpen = false
				m.paletteSelected = 0
				return m, nil
			}
			if m.input != "" {
				m.input = ""
				return m, nil
			}
			if m.commandRunning && m.cancel != nil {
				m.cancel()
				m.cancel = nil
				m.commandRunning = false
				m.stream = nil
				m.transcript = updateLastAgentTranscript(m.transcript, "\nstream cancelled")
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+c", "q":
			if m.input != "" && msg.String() == "q" {
				m.input += msg.String()
				m.syncPaletteState()
				return m, nil
			}
			if m.input != "" {
				m.input = ""
				m.syncPaletteState()
				return m, nil
			}
			if m.commandRunning && m.cancel != nil {
				m.cancel()
				m.cancel = nil
				m.commandRunning = false
				m.stream = nil
				m.transcript = updateLastAgentTranscript(m.transcript, "\nstream cancelled")
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			command := strings.TrimSpace(m.input)
			if m.paletteOpen {
				command = m.selectedPaletteCommand(command)
			}
			if command == "" || m.commandRunning {
				return m, nil
			}
			m.input = ""
			m.paletteOpen = false
			m.paletteSelected = 0
			m.commandRunning = true
			m.transcriptScroll = 0
			m.transcript = appendTranscript(m.transcript, "you", command)
			if !strings.HasPrefix(command, "/") {
				ch, cancel := startConsoleChatStream(m.opts, m.effectiveRunID(), command)
				m.stream = ch
				m.cancel = cancel
				m.transcript = append(m.transcript, ui.ConsoleTranscriptItem{Role: "agent"})
				return m, waitConsoleStream(ch)
			}
			return m, executeConsoleCommandCmd(m.client, m.opts, m.effectiveRunID(), command)
		case "backspace":
			m.input = dropLastRune(m.input)
			m.syncPaletteState()
			return m, nil
		case "up", "k":
			if m.paletteOpen {
				if m.paletteSelected > 0 {
					m.paletteSelected--
				}
				return m, nil
			}
			if m.input == "" {
				m.transcriptScroll += 1
				return m, nil
			}
			if msg.String() == "k" {
				m.input += msg.String()
				m.syncPaletteState()
				return m, nil
			}
		case "down", "j":
			if m.paletteOpen {
				if count := len(m.paletteMatches()); count > 0 && m.paletteSelected < count-1 {
					m.paletteSelected++
				}
				return m, nil
			}
			if m.input == "" {
				if m.transcriptScroll > 0 {
					m.transcriptScroll -= 1
				}
				return m, nil
			}
			if msg.String() == "j" {
				m.input += msg.String()
				m.syncPaletteState()
				return m, nil
			}
		case "tab":
			if m.paletteOpen {
				m.input = m.selectedPaletteCommand(m.input)
				m.syncPaletteState()
				return m, nil
			}
		case "pgup":
			if m.input == "" {
				m.transcriptScroll += 8
				return m, nil
			}
		case "pgdown":
			if m.input == "" {
				m.transcriptScroll -= 8
				if m.transcriptScroll < 0 {
					m.transcriptScroll = 0
				}
				return m, nil
			}
		case "home":
			if m.input == "" {
				m.transcriptScroll = len(m.transcript) * 8
				return m, nil
			}
		case "end":
			if m.input == "" {
				m.transcriptScroll = 0
				return m, nil
			}
		case "r":
			if m.input != "" {
				m.input += msg.String()
				m.syncPaletteState()
				return m, nil
			}
			m.loading = true
			m.message = "refreshing"
			return m, fetchConsoleViewCmd(m.client, m.opts)
		case "?":
			if m.input != "" {
				m.input += msg.String()
				m.syncPaletteState()
			} else {
				m.help = !m.help
			}
			return m, nil
		default:
			if len(msg.Runes) > 0 {
				m.input += string(msg.Runes)
				m.syncPaletteState()
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case consoleViewMsg:
		m.view = msg.view
		m.err = msg.err
		m.loading = false
		if msg.err != nil {
			m.message = msg.err.Error()
		} else {
			m.message = "updated " + time.Now().UTC().Format(time.RFC3339)
		}
		return m, consoleTick(m.opts.refreshEvery)
	case consoleTickMsg:
		if m.commandRunning {
			return m, consoleTick(m.opts.refreshEvery)
		}
		m.loading = true
		return m, fetchConsoleViewCmd(m.client, m.opts)
	case consoleCommandMsg:
		m.commandRunning = false
		m.cancel = nil
		m.stream = nil
		m.transcriptScroll = 0
		if msg.err != nil {
			m.transcript = appendTranscript(m.transcript, "error", msg.err.Error())
		} else if strings.TrimSpace(msg.out) != "" {
			m.transcript = appendTranscript(m.transcript, "agent", normalizeConsoleCommandOutput(msg.out))
		}
		return m, fetchConsoleViewCmd(m.client, m.opts)
	case consoleStreamMsg:
		if msg.delta != "" {
			m.transcript = updateLastAgentTranscript(m.transcript, msg.delta)
		}
		if msg.err != nil {
			m.transcript = updateLastErrorTranscript(m.transcript, msg.err.Error())
			m.commandRunning = false
			m.cancel = nil
			m.stream = nil
			m.transcriptScroll = 0
			return m, fetchConsoleViewCmd(m.client, m.opts)
		}
		if msg.done {
			m.commandRunning = false
			m.cancel = nil
			m.stream = nil
			m.transcriptScroll = 0
			return m, fetchConsoleViewCmd(m.client, m.opts)
		}
		return m, waitConsoleStream(m.stream)
	}
	return m, nil
}

func (m consoleTUIModel) View() string {
	if m.view.Server == "" {
		m.view = ui.ConsoleView{
			Server:       strings.TrimRight(m.opts.serverURL, "/"),
			Watch:        true,
			RefreshEvery: m.opts.refreshEvery,
			Warnings:     []string{m.message},
		}
	}
	status := m.message
	if m.loading {
		status = "refreshing"
	}
	if m.commandRunning {
		status = "running"
	}
	if m.err != nil {
		status = "last error: " + m.err.Error()
	}
	return ui.RenderConsoleWorkspace(ui.ConsoleWorkspace{
		View:             m.view,
		RunID:            m.effectiveRunID(),
		Input:            m.input,
		Help:             m.help,
		Running:          m.commandRunning,
		Status:           status,
		Transcript:       m.transcript,
		TranscriptScroll: m.transcriptScroll,
		Palette:          m.consolePaletteRows(),
		PaletteSelected:  m.paletteSelected,
		Width:            m.width,
		Height:           m.height,
	})
}

func (m consoleTUIModel) effectiveRunID() string {
	if strings.TrimSpace(m.opts.runID) != "" {
		return strings.TrimSpace(m.opts.runID)
	}
	if m.view.ActiveRun != nil {
		return strings.TrimSpace(m.view.ActiveRun.ID)
	}
	if len(m.view.Runs) > 0 {
		return strings.TrimSpace(m.view.Runs[0].ID)
	}
	return ""
}

func (m *consoleTUIModel) syncPaletteState() {
	m.paletteOpen = strings.HasPrefix(strings.TrimSpace(m.input), "/")
	if !m.paletteOpen {
		m.paletteSelected = 0
		return
	}
	count := len(m.paletteMatches())
	if count == 0 || m.paletteSelected >= count {
		m.paletteSelected = 0
	}
}

func (m consoleTUIModel) paletteMatches() []slashCommandMatch {
	return matchSlashCommands(m.input)
}

func (m consoleTUIModel) selectedPaletteCommand(input string) string {
	matches := m.paletteMatches()
	if len(matches) == 0 {
		return strings.TrimSpace(input)
	}
	selected := m.paletteSelected
	if selected < 0 || selected >= len(matches) {
		selected = 0
	}
	return completeSlashCommandInput(input, matches[selected].Command.Name)
}

func (m consoleTUIModel) consolePaletteRows() []ui.ConsolePaletteRow {
	if !m.paletteOpen {
		return nil
	}
	matches := m.paletteMatches()
	rows := make([]ui.ConsolePaletteRow, 0, len(matches))
	hasRun := m.effectiveRunID() != ""
	for _, match := range matches {
		row := ui.ConsolePaletteRow{
			Label:       match.Command.Name,
			Usage:       match.Command.Usage,
			Description: match.Command.Description,
			Disabled:    match.Command.RequiresRun && !hasRun,
			Match:       match.Indices,
		}
		if row.Disabled {
			row.Annotation = "needs run"
		}
		rows = append(rows, row)
	}
	return rows
}

func matchSlashCommands(input string) []slashCommandMatch {
	query := slashCommandQuery(input)
	matches := make([]slashCommandMatch, 0, len(slashCommands))
	for _, command := range slashCommands {
		indices, ok := fuzzyCommandMatch(command.Name, query)
		if !ok {
			for _, alias := range command.Aliases {
				if _, aliasOK := fuzzyCommandMatch(alias, query); aliasOK {
					indices, ok = fuzzyCommandMatch(command.Name, "")
					break
				}
			}
		}
		if ok {
			matches = append(matches, slashCommandMatch{Command: command, Indices: indices})
		}
	}
	return matches
}

func slashCommandQuery(input string) string {
	input = strings.TrimSpace(input)
	if input == "" || input == "/" {
		return ""
	}
	first, _, _ := strings.Cut(input, " ")
	return strings.TrimPrefix(strings.ToLower(first), "/")
}

func fuzzyCommandMatch(name string, query string) ([]int, bool) {
	query = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(query)), "/")
	label := strings.TrimPrefix(strings.ToLower(name), "/")
	if query == "" {
		return nil, true
	}
	if !strings.HasPrefix(label, query) {
		return nil, false
	}
	indices := make([]int, 0, len(query))
	for i := 0; i < len(query); i++ {
		indices = append(indices, i+1)
	}
	return indices, true
}

func completeSlashCommandInput(input string, command string) string {
	input = strings.TrimSpace(input)
	if input == "" || input == "/" {
		return command
	}
	first, rest, hasRest := strings.Cut(input, " ")
	if !strings.HasPrefix(first, "/") {
		return input
	}
	if hasRest {
		return command + " " + strings.TrimSpace(rest)
	}
	return command
}

func fetchConsoleViewCmd(client *http.Client, opts tuiCommandOptions) tea.Cmd {
	return func() tea.Msg {
		view, err := remoteConsoleView(client, opts)
		return consoleViewMsg{view: view, err: err}
	}
}

func consoleTick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return consoleTickMsg(t)
	})
}

func startConsoleChatStream(opts tuiCommandOptions, runID string, input string) (<-chan consoleStreamMsg, context.CancelFunc) {
	ch := make(chan consoleStreamMsg, 16)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(ch)
		responder, err := consoleChatResponder(opts, runID)
		if err != nil {
			ch <- consoleStreamMsg{err: err}
			return
		}
		err = responder.StreamRespond(ctx, input, func(delta string) error {
			if delta == "" {
				return nil
			}
			select {
			case ch <- consoleStreamMsg{delta: delta}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			ch <- consoleStreamMsg{err: err}
			return
		}
		ch <- consoleStreamMsg{done: true}
	}()
	return ch, cancel
}

func consoleChatResponder(opts tuiCommandOptions, runID string) (ui.ChatResponder, error) {
	return &remoteRunChatResponder{
		serverURL:  strings.TrimRight(opts.serverURL, "/"),
		runID:      strings.TrimSpace(runID),
		httpClient: operatorStreamHTTPClient(),
	}, nil
}

func waitConsoleStream(ch <-chan consoleStreamMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return consoleStreamMsg{done: true}
		}
		return msg
	}
}

func executeConsoleCommandCmd(client *http.Client, opts tuiCommandOptions, runID string, input string) tea.Cmd {
	return func() tea.Msg {
		out, err := executeConsoleCommand(client, opts, runID, input)
		return consoleCommandMsg{input: input, out: out, err: err}
	}
}

func executeConsoleCommand(client *http.Client, opts tuiCommandOptions, runID string, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}
	serverURL := strings.TrimRight(opts.serverURL, "/")
	if strings.HasPrefix(input, "/") {
		var out bytes.Buffer
		handled, err := chatCommandHandlerWithClient(serverURL, runID, client)(context.Background(), input, &out, ui.ChatContext{ServerURL: serverURL, RunID: runID}, ui.ChatOptions{Plain: true})
		if !handled && err == nil {
			err = fmt.Errorf("unknown command %q; use /help", input)
		}
		return out.String(), err
	}
	if runID == "" {
		return "", fmt.Errorf("model chat needs an active run; use /sessions first or start TUI with --run <run-id>")
	}
	var out strings.Builder
	responder := &remoteRunChatResponder{
		serverURL:  serverURL,
		runID:      runID,
		httpClient: operatorStreamHTTPClient(),
	}
	err := responder.StreamRespond(context.Background(), input, func(delta string) error {
		out.WriteString(delta)
		return nil
	})
	return out.String(), err
}

func appendTranscript(items []ui.ConsoleTranscriptItem, role string, text string) []ui.ConsoleTranscriptItem {
	text = strings.TrimSpace(text)
	if text == "" {
		return items
	}
	items = append(items, ui.ConsoleTranscriptItem{Role: role, Text: text})
	if len(items) > 80 {
		items = items[len(items)-80:]
	}
	return items
}

func updateLastAgentTranscript(items []ui.ConsoleTranscriptItem, delta string) []ui.ConsoleTranscriptItem {
	if len(items) == 0 || strings.ToLower(strings.TrimSpace(items[len(items)-1].Role)) != "agent" {
		return appendTranscript(items, "agent", delta)
	}
	items = append([]ui.ConsoleTranscriptItem(nil), items...)
	items[len(items)-1].Text += delta
	return items
}

func updateLastErrorTranscript(items []ui.ConsoleTranscriptItem, text string) []ui.ConsoleTranscriptItem {
	text = strings.TrimSpace(text)
	if text == "" {
		return items
	}
	if len(items) > 0 && strings.ToLower(strings.TrimSpace(items[len(items)-1].Role)) == "agent" && strings.TrimSpace(items[len(items)-1].Text) == "" {
		items = append([]ui.ConsoleTranscriptItem(nil), items...)
		items[len(items)-1] = ui.ConsoleTranscriptItem{Role: "error", Text: text}
		return items
	}
	return appendTranscript(items, "error", text)
}

func normalizeConsoleCommandOutput(value string) string {
	value = strings.TrimSpace(value)
	for {
		next := strings.TrimSpace(strings.TrimPrefix(value, "agent:"))
		if next == value {
			return value
		}
		value = next
	}
}

func dropLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}

func parseTUIArgs(args []string) tuiCommandOptions {
	opts := tuiCommandOptions{
		serverURL:      "http://localhost:8080",
		watch:          true,
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
			opts.once = false
		case "--once":
			opts.watch = false
			opts.once = true
			opts.clearOnRefresh = false
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
