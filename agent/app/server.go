package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/Y4NN777/7review/agent/channels"
	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/jobs"
	"github.com/Y4NN777/7review/agent/memory"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/policy"
	"github.com/Y4NN777/7review/agent/publisher"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/skills"
	"github.com/Y4NN777/7review/agent/validator"
)

// Server wires HTTP routes to the review pipeline.
type Server struct {
	cfg      *config.Config
	pipeline *pipeline.Pipeline
	mux      *http.ServeMux
}

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
			Jobs:             jobs.NewMemoryStore(),
			Policy:           policy.DefaultFilter{},
			FindingValidator: validator.DefaultFindingValidator{},
			Publisher:        publisher.NoopPublisher{},
			Memory:           memory.NoopStore{},
		},
		mux: http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	log.Printf("[server] 7review listening on %s", s.cfg.ListenAddr)
	return http.ListenAndServe(s.cfg.ListenAddr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/webhook", channels.GitLabWebhookHandler(
		s.cfg.WebhookSecret,
		func(req review.Request) {
			ctx := context.Background()
			if err := s.pipeline.Run(ctx, req); err != nil {
				log.Printf("[server] pipeline error project=%s MR=!%d: %v", req.ProjectID, req.MRIID, err)
			}
		},
	))

	s.mux.HandleFunc("/approve", s.handleApprove)
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project")
	mrIID := 0
	if _, err := fmt.Sscanf(r.URL.Query().Get("mr"), "%d", &mrIID); err != nil || projectID == "" || mrIID == 0 {
		http.Error(w, "missing project or mr param", http.StatusBadRequest)
		return
	}

	body, _ := io.ReadAll(r.Body)
	approvedReport := string(body)

	go func() {
		ctx := context.Background()
		if err := s.pipeline.RunPostHIL(ctx, projectID, mrIID, approvedReport); err != nil {
			log.Printf("[server] post-HIL error project=%s MR=!%d: %v", projectID, mrIID, err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}
