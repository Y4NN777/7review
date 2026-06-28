package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/pipeline"
)

func TestHandleReadyReportsRequiredDependencies(t *testing.T) {
	s := &Server{
		pipeline: &pipeline.Pipeline{
			ContextReducer: fakeReducer{err: errors.New("headroom down")},
			Memory:         fakeMemory{},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	s.handleReady(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var status readinessStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.Ready || status.Dependencies["headroom"] == "ok" || status.Dependencies["mempalace"] != "ok" {
		t.Fatalf("unexpected readiness: %#v", status)
	}
}

func TestHandleReadyReportsHealthyRuntime(t *testing.T) {
	orch := orchestrator.NewOrchestrator(orchestrator.DefaultOrchestratorConfig("large", "small", "fake"), map[string]orchestrator.LLMProvider{
		"fake": streamingProvider{},
	})
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Orchestrator:     orch,
			Jobs:             pipeline.NewMemoryRunStore(),
			ContextReducer:   fakeReducer{},
			Memory:           fakeMemory{},
			SCM:              fakeAppSCM{},
			FindingValidator: pipeline.DefaultFindingValidator{},
		},
		work: make(chan workItem, 3),
	}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	s.handleReady(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var status readinessStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	for _, dep := range []string{"pipeline", "orchestrator", "queue", "run_store", "headroom", "mempalace"} {
		if status.Dependencies[dep] == "" {
			t.Fatalf("missing dependency %q in %#v", dep, status)
		}
	}
	if status.Queue.Depth != 0 || status.Queue.Capacity != 3 || status.Queue.Available != 3 {
		t.Fatalf("unexpected queue status: %#v", status.Queue)
	}
	if !status.Ready {
		t.Fatalf("expected ready status, got %#v", status)
	}
}

func TestHandleReadyReportsMissingCoreDependencies(t *testing.T) {
	s := &Server{pipeline: &pipeline.Pipeline{}}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	s.handleReady(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	var status readinessStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	for _, dep := range []string{"orchestrator", "queue", "run_store", "headroom", "mempalace"} {
		if status.Dependencies[dep] == "" || status.Dependencies[dep] == "ok" {
			t.Fatalf("expected dependency %q down in %#v", dep, status)
		}
	}
}

func TestHandleReadyMethod(t *testing.T) {
	s := &Server{pipeline: &pipeline.Pipeline{}}
	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	rec := httptest.NewRecorder()

	s.handleReady(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	s := &Server{
		cfg: &config.Config{
			ListenAddr:        ":9090",
			ReadHeaderTimeout: 7000,
			ReadTimeout:       31000,
			WriteTimeout:      121000,
			IdleTimeout:       122000,
		},
		mux: http.NewServeMux(),
	}

	server := s.httpServer()

	if server.Addr != ":9090" || server.Handler != s.mux {
		t.Fatalf("unexpected server wiring: %#v", server)
	}
	if server.ReadHeaderTimeout != 7*time.Second ||
		server.ReadTimeout != 31*time.Second ||
		server.WriteTimeout != 121*time.Second ||
		server.IdleTimeout != 122*time.Second {
		t.Fatalf("unexpected timeouts: header=%s read=%s write=%s idle=%s", server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout)
	}
}

func TestRunWorkItemCancelsAtConfiguredTimeout(t *testing.T) {
	s := &Server{cfg: &config.Config{WebhookJobTimeout: 1}}

	err := s.runWorkItem(1, workItem{
		name: "timeout",
		run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestRunWorkItemConvertsPanicToError(t *testing.T) {
	s := &Server{cfg: &config.Config{WebhookJobTimeout: 1000}}

	err := s.runWorkItem(1, workItem{
		name: "panic-job",
		run: func(ctx context.Context) error {
			panic("broken adapter")
		},
	})

	if err == nil || !strings.Contains(err.Error(), "panic processing panic-job") {
		t.Fatalf("expected panic error, got %v", err)
	}
}

func TestQueueStatusTracksWorkerOutcomes(t *testing.T) {
	s := &Server{
		cfg:  &config.Config{WebhookJobTimeout: 1000},
		work: make(chan workItem, 2),
	}
	if err := s.enqueue(workItem{name: "ok", run: func(context.Context) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if got := s.queueStatus(); got.Depth != 1 || got.Capacity != 2 || got.Available != 1 || got.Enqueued != 1 {
		t.Fatalf("unexpected queued status: %#v", got)
	}
	item := <-s.work
	if err := s.runWorkItem(1, item); err != nil {
		t.Fatal(err)
	}
	s.stats.completed.Add(1)
	if err := s.runWorkItem(1, workItem{name: "bad", run: func(context.Context) error { return errors.New("boom") }}); err == nil {
		t.Fatal("expected worker error")
	}
	s.stats.failed.Add(1)

	got := s.queueStatus()
	if got.Depth != 0 || got.Completed != 1 || got.Failed != 1 || got.Enqueued != 1 {
		t.Fatalf("unexpected final queue status: %#v", got)
	}
}

func TestRequireAuthRejectsMissingOperatorToken(t *testing.T) {
	s := &Server{cfg: &config.Config{APIToken: "secret"}}
	req := httptest.NewRequest(http.MethodGet, "/runs", nil)
	rec := httptest.NewRecorder()

	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthAcceptsBearerToken(t *testing.T) {
	s := &Server{cfg: &config.Config{APIToken: "secret"}}
	req := httptest.NewRequest(http.MethodGet, "/runs", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestClaimDeliveryDeduplicatesAndCanRelease(t *testing.T) {
	s := &Server{}
	if !s.claimDelivery("github:delivery-1") {
		t.Fatal("first delivery should be accepted")
	}
	if s.claimDelivery("github:delivery-1") {
		t.Fatal("duplicate delivery should be rejected")
	}
	s.releaseDelivery("github:delivery-1")
	if !s.claimDelivery("github:delivery-1") {
		t.Fatal("released delivery should be accepted again")
	}
}

func TestClaimDeliveryPurgesExpiredEntries(t *testing.T) {
	s := &Server{
		seen: map[string]time.Time{
			"github:old": time.Now().UTC().Add(-deliveryRetention - time.Minute),
		},
	}

	if !s.claimDelivery("github:old") {
		t.Fatal("expired delivery should be accepted again")
	}
	if len(s.seen) != 1 {
		t.Fatalf("expired delivery map was not compacted: %#v", s.seen)
	}
}

func TestDeliveryKeyReleasedWhenQueuedWebhookRunFails(t *testing.T) {
	s := &Server{
		cfg:      &config.Config{GitLabURL: "https://gitlab.example.com", GitLabToken: "token", WebhookSecret: "secret"},
		pipeline: &pipeline.Pipeline{},
		mux:      http.NewServeMux(),
		work:     make(chan workItem, 1),
		seen:     make(map[string]time.Time),
	}
	s.routes()
	body := `{
		"object_kind":"merge_request",
		"event_type":"merge_request",
		"project":{"id":42},
		"object_attributes":{"iid":7,"action":"update","title":"Fix auth"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(body))
	req.Header.Set("X-Gitlab-Token", "secret")
	req.Header.Set("X-Gitlab-Event-UUID", "delivery-fail")
	rec := httptest.NewRecorder()

	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	item := <-s.work
	if err := item.run(context.Background()); err == nil {
		t.Fatal("expected pipeline failure")
	}
	if !s.claimDelivery("gitlab:delivery-fail") {
		t.Fatal("failed delivery should be retryable")
	}
}
