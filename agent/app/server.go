package app

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/pipeline"
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
	s.pipeline.Memory = reviewMemoryStore(cfg)
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
