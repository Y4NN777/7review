package app

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/skills"
	"github.com/Y4NN777/7review/agent/tools"
)

// Server wires HTTP routes to the review pipeline.
type Server struct {
	cfg      *config.Config
	pipeline *pipeline.Pipeline
	mux      *http.ServeMux
	work     chan workItem
	seenMu   sync.Mutex
	seen     map[string]time.Time
	stats    workerStats
}

type workItem struct {
	name string
	run  func(context.Context) error
}

type workerStats struct {
	enqueued  atomic.Uint64
	completed atomic.Uint64
	failed    atomic.Uint64
}

const deliveryRetention = 24 * time.Hour
const defaultWebhookJobTimeout = 15 * time.Minute
const chatMaxBodyBytes int64 = 64 * 1024
const reportMaxBodyBytes int64 = 512 * 1024

// NewServer builds the application server and all runtime dependencies.
func NewServer() (*Server, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	sl := &skills.Loader{SkillsDir: cfg.SkillsDir}
	if err := sl.Load(); err != nil {
		return nil, fmt.Errorf("skills: %w", err)
	}

	modelOrchestrator, err := orchestrator.BuildOrchestrator(cfg)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: %w", err)
	}

	s := &Server{
		cfg: cfg,
		pipeline: &pipeline.Pipeline{
			Config:           cfg,
			SkillLoader:      sl,
			Orchestrator:     modelOrchestrator,
			Jobs:             pipeline.NewFileRunStore(cfg.MemoryDir + "/runs"),
			Policy:           pipeline.DefaultPolicyFilter{},
			FindingValidator: pipeline.DefaultFindingValidator{},
		},
		mux:  http.NewServeMux(),
		seen: make(map[string]time.Time),
	}
	s.work = make(chan workItem, cfg.WebhookQueueSize)
	gitlab := tools.NewGitLabClient(cfg.GitLabURL, cfg.GitLabToken)
	github := tools.NewGitHubClient(cfg.GitHubAPIURL, cfg.GitHubToken)
	router := tools.ProviderRouter{
		SCM: map[string]tools.SCM{
			"gitlab": gitlab,
			"github": github,
		},
		Publishers: map[string]tools.Publisher{
			"gitlab": gitlab,
			"github": github,
		},
	}
	s.pipeline.SCM = router
	s.pipeline.SCMPublisher = router
	s.pipeline.ContextReducer = tools.NewHeadroomReducer(cfg.HeadroomURL, time.Duration(cfg.HeadroomTimeout)*time.Millisecond)
	s.pipeline.Memory = tools.NewMemPalaceStore(cfg.MemPalaceURL, time.Duration(cfg.MemPalaceTimeout)*time.Millisecond)
	s.startWorkers()
	s.routes()
	return s, nil
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	log.Printf("[server] 7review listening on %s", s.cfg.ListenAddr)
	return s.httpServer().ListenAndServe()
}

func (s *Server) httpServer() *http.Server {
	cfg := &config.Config{ListenAddr: ":8080"}
	if s != nil && s.cfg != nil {
		cfg = s.cfg
	}
	return &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           s.mux,
		ReadHeaderTimeout: durationMS(cfg.ReadHeaderTimeout, 5*time.Second),
		ReadTimeout:       durationMS(cfg.ReadTimeout, 30*time.Second),
		WriteTimeout:      durationMS(cfg.WriteTimeout, 120*time.Second),
		IdleTimeout:       durationMS(cfg.IdleTimeout, 120*time.Second),
	}
}

func durationMS(value int, fallback time.Duration) time.Duration {
	if value > 0 {
		return time.Duration(value) * time.Millisecond
	}
	return fallback
}

func (s *Server) routes() {
	run := func(req review.Request) error {
		name := fmt.Sprintf("%s/%s/%s", req.Provider, req.ProjectID, req.ChangeID)
		deliveryKey := req.Provider + ":" + req.DeliveryID
		if req.DeliveryID != "" && !s.claimDelivery(deliveryKey) {
			log.Printf("[server] duplicate webhook delivery ignored: %s", deliveryKey)
			return nil
		}
		if err := s.enqueue(workItem{
			name: name,
			run: func(ctx context.Context) error {
				err := s.pipeline.Run(ctx, req)
				if err != nil && req.DeliveryID != "" {
					s.releaseDelivery(deliveryKey)
				}
				return err
			},
		}); err != nil {
			if req.DeliveryID != "" {
				s.releaseDelivery(deliveryKey)
			}
			return err
		}
		return nil
	}

	if s.gitLabWebhookConfigured() {
		s.mux.HandleFunc("/webhook", gitLabWebhookHandler(s.cfg.WebhookSecret, run))
		s.mux.HandleFunc("/webhook/gitlab", gitLabWebhookHandler(s.cfg.WebhookSecret, run))
	} else {
		s.mux.HandleFunc("/webhook", inactiveWebhookHandler("gitlab"))
		s.mux.HandleFunc("/webhook/gitlab", inactiveWebhookHandler("gitlab"))
	}
	if s.gitHubWebhookConfigured() {
		s.mux.HandleFunc("/webhook/github", gitHubWebhookHandler(s.cfg.GitHubWebhookSecret, run))
	} else {
		s.mux.HandleFunc("/webhook/github", inactiveWebhookHandler("github"))
	}

	s.mux.HandleFunc("/approve", s.requireAuth(s.handleApprove))
	s.mux.HandleFunc("/publish/final", s.requireAuth(s.handlePublishFinal))
	s.mux.HandleFunc("/runs", s.requireAuth(s.handleRuns))
	s.mux.HandleFunc("/run", s.requireAuth(s.handleRun))
	s.mux.HandleFunc("/chat/stream", s.requireAuth(s.handleChatStream))
	s.mux.HandleFunc("/tools", s.requireAuth(s.handleTools))
	s.mux.HandleFunc("/tools/execute", s.requireAuth(s.handleToolExecute))
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	s.mux.HandleFunc("/ready", s.requireAuth(s.handleReady))
}

func (s *Server) gitLabWebhookConfigured() bool {
	return s != nil && s.cfg != nil && s.cfg.GitLabURL != "" && s.cfg.GitLabToken != "" && s.cfg.WebhookSecret != ""
}

func (s *Server) gitHubWebhookConfigured() bool {
	return s != nil && s.cfg != nil && s.cfg.GitHubAPIURL != "" && s.cfg.GitHubToken != "" && s.cfg.GitHubWebhookSecret != ""
}

func inactiveWebhookHandler(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, provider+" webhook is not configured", http.StatusNotFound)
	}
}

func (s *Server) claimDelivery(key string) bool {
	if key == "" {
		return true
	}
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	if s.seen == nil {
		s.seen = make(map[string]time.Time)
	}
	s.purgeExpiredDeliveriesLocked(time.Now().UTC())
	if _, ok := s.seen[key]; ok {
		return false
	}
	s.seen[key] = time.Now().UTC()
	return true
}

func (s *Server) releaseDelivery(key string) {
	if key == "" {
		return
	}
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	delete(s.seen, key)
}

func (s *Server) purgeExpiredDeliveriesLocked(now time.Time) {
	for key, seenAt := range s.seen {
		if now.Sub(seenAt) > deliveryRetention {
			delete(s.seen, key)
		}
	}
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if s != nil && s.cfg != nil {
			token = s.cfg.APIToken
		}
		if token == "" {
			next(w, r)
			return
		}
		if !validBearerToken(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func validBearerToken(r *http.Request, expected string) bool {
	provided := strings.TrimSpace(r.Header.Get("X-7review-Token"))
	if provided == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			provided = strings.TrimSpace(auth[len("Bearer "):])
		}
	}
	if provided == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tools.Catalog())
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	runs, err := s.pipeline.Jobs.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]runDTO, 0, len(runs))
	for _, run := range runs {
		out = append(out, toRunDTO(run, false))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	run, err := s.pipeline.Jobs.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toRunDTO(*run, true))
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("run")
	if id == "" {
		http.Error(w, "missing run", http.StatusBadRequest)
		return
	}
	run, err := s.pipeline.Jobs.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var req chatStreamRequest
	body, err := readBoundedBody(r.Body, chatMaxBodyBytes)
	if err != nil {
		http.Error(w, "chat message too large", http.StatusRequestEntityTooLarge)
		return
	}
	if len(body) > 0 && strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid chat request", http.StatusBadRequest)
			return
		}
	} else {
		req.Message = strings.TrimSpace(string(body))
	}
	if req.Message == "" {
		http.Error(w, "missing message", http.StatusBadRequest)
		return
	}
	if s.pipeline.Orchestrator == nil {
		http.Error(w, "orchestrator is not configured", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	rc := run.Context
	if rc == nil {
		rc = review.NewContext(run.Request)
		rc.Findings = append([]review.Finding(nil), run.Findings...)
		rc.DraftReport = run.DraftReport
		rc.FinalReport = run.FinalReport
		rc.WebURL = run.WebURL
	}
	system := reviewChatSystemPrompt(*run)
	user := fmt.Sprintf("Engineer message:\n%s", req.Message)
	_ = s.pipeline.Jobs.AppendEvent(r.Context(), id, pipeline.RunEvent{
		Type:    "chat_message",
		Status:  run.Status,
		Message: truncateEventMessage(req.Message),
		Meta: map[string]string{
			"role":  "engineer",
			"bytes": fmt.Sprintf("%d", len(req.Message)),
		},
	})
	responseText, err := s.pipeline.Orchestrator.StreamComplete(r.Context(), rc, orchestrator.RoleFormatter, "review_chat", system, user, func(delta string) error {
		data, _ := json.Marshal(chatStreamEvent{Delta: delta})
		if _, writeErr := fmt.Fprintf(w, "data: %s\n\n", data); writeErr != nil {
			return writeErr
		}
		flusher.Flush()
		return nil
	})
	if err != nil {
		_ = s.pipeline.Jobs.AppendEvent(r.Context(), id, pipeline.RunEvent{
			Type:    "chat_failed",
			Status:  run.Status,
			Message: truncateEventMessage(err.Error()),
			Meta:    map[string]string{"role": "agent"},
		})
		data, _ := json.Marshal(chatStreamEvent{Error: err.Error()})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
		flusher.Flush()
		return
	}
	_ = s.pipeline.Jobs.AppendEvent(r.Context(), id, pipeline.RunEvent{
		Type:    "chat_response",
		Status:  run.Status,
		Message: truncateEventMessage(responseText),
		Meta: map[string]string{
			"role":  "agent",
			"bytes": fmt.Sprintf("%d", len(responseText)),
		},
	})
	_, _ = fmt.Fprint(w, "event: done\ndata: {}\n\n")
	flusher.Flush()
}

func truncateEventMessage(message string) string {
	message = strings.TrimSpace(message)
	const maxEventMessageBytes = 2000
	if len(message) <= maxEventMessageBytes {
		return message
	}
	return message[:maxEventMessageBytes] + "..."
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	runID := strings.TrimSpace(r.URL.Query().Get("run"))
	projectID := strings.TrimSpace(r.URL.Query().Get("project"))
	mrIID := 0
	if runID == "" {
		if _, err := fmt.Sscanf(r.URL.Query().Get("mr"), "%d", &mrIID); err != nil || projectID == "" || mrIID == 0 {
			http.Error(w, "missing run or project/mr param", http.StatusBadRequest)
			return
		}
		runID = fmt.Sprintf("%s!%d", projectID, mrIID)
	}

	body, err := readBoundedBody(r.Body, reportMaxBodyBytes)
	if err != nil {
		http.Error(w, "approval report too large", http.StatusRequestEntityTooLarge)
		return
	}
	approvedReport := string(body)

	if err := s.enqueue(workItem{
		name: fmt.Sprintf("approve/%s", runID),
		run: func(ctx context.Context) error {
			return s.pipeline.ApproveRun(ctx, runID, approvedReport)
		},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePublishFinal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("run")
	if id == "" {
		http.Error(w, "missing run", http.StatusBadRequest)
		return
	}
	body, err := readBoundedBody(r.Body, reportMaxBodyBytes)
	if err != nil {
		http.Error(w, "final report too large", http.StatusRequestEntityTooLarge)
		return
	}
	report := string(body)
	if err := s.enqueue(workItem{
		name: fmt.Sprintf("publish/final/%s", id),
		run: func(ctx context.Context) error {
			return s.pipeline.PublishFinal(ctx, id, report)
		},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) startWorkers() {
	workers := s.cfg.WebhookWorkers
	if workers <= 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		workerID := i + 1
		go func() {
			for item := range s.work {
				if err := s.runWorkItem(workerID, item); err != nil {
					s.stats.failed.Add(1)
					log.Printf("[worker %d] failed %s: %v", workerID, item.name, err)
				} else {
					s.stats.completed.Add(1)
				}
			}
		}()
	}
}

func (s *Server) runWorkItem(workerID int, item workItem) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.webhookJobTimeout())
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic processing %s: %v", item.name, recovered)
		}
	}()
	return item.run(ctx)
}

func (s *Server) webhookJobTimeout() time.Duration {
	if s != nil && s.cfg != nil && s.cfg.WebhookJobTimeout > 0 {
		return time.Duration(s.cfg.WebhookJobTimeout) * time.Millisecond
	}
	return defaultWebhookJobTimeout
}

func (s *Server) enqueue(item workItem) error {
	select {
	case s.work <- item:
		s.stats.enqueued.Add(1)
		return nil
	default:
		return fmt.Errorf("review queue is full")
	}
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status := s.readiness(ctx)

	w.Header().Set("Content-Type", "application/json")
	if !status.Ready {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(status)
}

type readinessStatus struct {
	Ready        bool              `json:"ready"`
	Dependencies map[string]string `json:"dependencies"`
	Queue        queueStatus       `json:"queue,omitempty"`
}

func (s *readinessStatus) markDown(name, message string) {
	s.Ready = false
	s.Dependencies[name] = message
}

type queueStatus struct {
	Depth     int    `json:"depth"`
	Capacity  int    `json:"capacity"`
	Available int    `json:"available"`
	Enqueued  uint64 `json:"enqueued"`
	Completed uint64 `json:"completed"`
	Failed    uint64 `json:"failed"`
}

func (s *Server) queueStatus() queueStatus {
	if s == nil || s.work == nil {
		return queueStatus{}
	}
	depth := len(s.work)
	capacity := cap(s.work)
	return queueStatus{
		Depth:     depth,
		Capacity:  capacity,
		Available: capacity - depth,
		Enqueued:  s.stats.enqueued.Load(),
		Completed: s.stats.completed.Load(),
		Failed:    s.stats.failed.Load(),
	}
}

type runDTO struct {
	ID          string              `json:"id"`
	Provider    string              `json:"provider"`
	ProjectID   string              `json:"project_id"`
	ChangeID    string              `json:"change_id"`
	MRIID       int                 `json:"mr_iid"`
	Title       string              `json:"title,omitempty"`
	Status      pipeline.RunStatus  `json:"status"`
	Error       string              `json:"error,omitempty"`
	WebURL      string              `json:"web_url,omitempty"`
	UpdatedAt   time.Time           `json:"updated_at"`
	EventCount  int                 `json:"event_count"`
	Events      []pipeline.RunEvent `json:"events,omitempty"`
	Findings    []review.Finding    `json:"findings,omitempty"`
	DraftReport string              `json:"draft_report,omitempty"`
	FinalReport string              `json:"final_report,omitempty"`
	HILApproved bool                `json:"hil_approved"`
}

type chatStreamRequest struct {
	Message string `json:"message"`
}

type chatStreamEvent struct {
	Delta string `json:"delta,omitempty"`
	Error string `json:"error,omitempty"`
}

func toRunDTO(run pipeline.Run, includeDetails bool) runDTO {
	dto := runDTO{
		ID:          run.ID,
		Provider:    run.Request.Provider,
		ProjectID:   run.Request.ProjectID,
		ChangeID:    run.Request.ChangeID,
		MRIID:       run.Request.MRIID,
		Title:       run.Request.Title,
		Status:      run.Status,
		Error:       run.Error,
		WebURL:      run.WebURL,
		UpdatedAt:   run.UpdatedAt,
		EventCount:  len(run.Events),
		HILApproved: run.HILApproved,
	}
	if includeDetails {
		dto.Events = append([]pipeline.RunEvent(nil), run.Events...)
		dto.Findings = run.Findings
		dto.DraftReport = run.DraftReport
		dto.FinalReport = run.FinalReport
	}
	return dto
}

func reviewChatSystemPrompt(run pipeline.Run) string {
	var b strings.Builder
	b.WriteString(strings.Join([]string{
		"You are 7review's live review copilot for one concrete PR/MR review run.",
		"You are talking to an engineer during iterative review, before or during HIL.",
		"Use only the run facts provided below plus the engineer's message.",
		"Do not invent files, findings, approvals, CI status, SCM comments, memory writes, or dependency health.",
		"Always distinguish known facts from assumptions.",
		"When discussing a finding, explain: risk, evidence from the stored finding/report, what would prove it false, and the next useful action.",
		"When the engineer asks what to do next, provide one explicit next command or endpoint.",
		"When HIL is not approved, do not say the review is final and do not propose writing memory as complete.",
		"When the draft has no findings, help verify coverage instead of claiming correctness.",
		"Keep responses concise and engineering-focused.",
	}, "\n"))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Run ID: %s\nStatus: %s\nProvider: %s\nProject: %s\nChange: %s\nURL: %s\n",
		run.ID, run.Status, run.Request.Provider, run.Request.ProjectID, run.Request.ChangeID, run.WebURL)
	if run.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n", run.Error)
	}
	if len(run.Findings) > 0 {
		b.WriteString("\nValidated findings:\n")
		for _, finding := range run.Findings {
			fmt.Fprintf(&b, "- %s %s: %s", finding.ID, finding.Severity, finding.Title)
			if finding.Location.Path != "" {
				fmt.Fprintf(&b, " (%s:%d)", finding.Location.Path, finding.Location.Line)
			}
			b.WriteString("\n")
		}
	}
	if history := renderRecentRunEvents(run.Events, 8); history != "" {
		b.WriteString("\nRecent run events:\n")
		b.WriteString(history)
	}
	if run.DraftReport != "" {
		b.WriteString("\nDraft report:\n")
		b.WriteString(run.DraftReport)
	}
	return b.String()
}

func renderRecentRunEvents(events []pipeline.RunEvent, limit int) string {
	if limit <= 0 || len(events) == 0 {
		return ""
	}
	start := len(events) - limit
	if start < 0 {
		start = 0
	}
	var lines []string
	for _, event := range events[start:] {
		eventType := strings.TrimSpace(event.Type)
		if eventType == "" {
			eventType = "event"
		}
		parts := []string{eventType}
		if event.Status != "" {
			parts = append(parts, string(event.Status))
		}
		if message := truncatePromptEventMessage(event.Message); message != "" {
			parts = append(parts, message)
		}
		if role := strings.TrimSpace(event.Meta["role"]); role != "" {
			parts = append(parts, "role="+role)
		}
		lines = append(lines, "- "+strings.Join(parts, " | "))
	}
	return strings.Join(lines, "\n") + "\n"
}

func truncatePromptEventMessage(message string) string {
	message = strings.TrimSpace(message)
	const maxPromptEventMessage = 240
	if len(message) <= maxPromptEventMessage {
		return message
	}
	if maxPromptEventMessage <= 3 {
		return message[:maxPromptEventMessage]
	}
	return message[:maxPromptEventMessage-3] + "..."
}
