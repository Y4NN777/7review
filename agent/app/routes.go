package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
)

func (s *Server) routes() {
	if s.gitLabWebhookConfigured() {
		s.mux.HandleFunc("/webhook", gitLabWebhookHandler(s.cfg.WebhookSecret, s.handleWebhookReview))
		s.mux.HandleFunc("/webhook/gitlab", gitLabWebhookHandler(s.cfg.WebhookSecret, s.handleWebhookReview))
	} else {
		s.mux.HandleFunc("/webhook", inactiveWebhookHandler("gitlab"))
		s.mux.HandleFunc("/webhook/gitlab", inactiveWebhookHandler("gitlab"))
	}
	if s.gitHubWebhookConfigured() {
		s.mux.HandleFunc("/webhook/github", gitHubWebhookHandler(s.cfg.GitHubWebhookSecret, s.handleWebhookReview))
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

type reviewDispatchResult struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type requestReviewResult struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func (s *Server) handleWebhookReview(req review.Request) (reviewDispatchResult, error) {
	decision := s.reviewPolicyDecision(req)
	runID := requestRunID(req)
	if !decision.allowed {
		log.Printf("[server] webhook review ignored by policy: run=%s reason=%s", runID, decision.reason)
		return reviewDispatchResult{RunID: runID, Status: "ignored", Reason: "ignored by review policy: " + decision.reason}, nil
	}
	result, err := s.enqueueReview(req, true)
	return reviewDispatchResult{RunID: result.RunID, Status: result.Status, Reason: result.Reason}, err
}

func (s *Server) requestReview(ctx context.Context, req review.Request) (requestReviewResult, error) {
	req.DeliveryID = ""
	req.EventAction = "manual"
	result, err := s.enqueueReview(req, false)
	return result, err
}

func (s *Server) enqueueReview(req review.Request, webhook bool) (requestReviewResult, error) {
	runID := requestRunID(req)
	if runID == "" {
		return requestReviewResult{Status: "rejected", Reason: "review request is missing run identity"}, fmt.Errorf("review request is missing run identity")
	}
	if s.isRunActive(runID) {
		return requestReviewResult{RunID: runID, Status: "rejected", Reason: "review already running"}, fmt.Errorf("review already running for %s", runID)
	}
	if s.pipeline != nil && s.pipeline.Jobs != nil {
		if run, err := s.pipeline.Jobs.Get(context.Background(), runID); err == nil && run.Status == pipeline.StatusRunning {
			return requestReviewResult{RunID: runID, Status: "rejected", Reason: "review already running"}, fmt.Errorf("review already running for %s", runID)
		}
	}
	name := fmt.Sprintf("%s/%s/%s", req.Provider, req.ProjectID, firstNonEmptyString(req.ChangeID, strconv.Itoa(req.MRIID)))
	deliveryKey := req.Provider + ":" + req.DeliveryID
	if webhook && req.DeliveryID != "" && !s.claimDelivery(deliveryKey) {
		log.Printf("[server] duplicate webhook delivery ignored: %s", deliveryKey)
		return requestReviewResult{RunID: runID, Status: "ignored", Reason: "duplicate webhook delivery"}, nil
	}
	s.markRunActive(runID)
	if err := s.enqueue(workItem{
		name: name,
		run: func(ctx context.Context) error {
			defer s.clearRunActive(runID)
			err := s.pipeline.Run(ctx, req)
			if err != nil && webhook && req.DeliveryID != "" {
				s.releaseDelivery(deliveryKey)
			}
			return err
		},
	}); err != nil {
		s.clearRunActive(runID)
		if webhook && req.DeliveryID != "" {
			s.releaseDelivery(deliveryKey)
		}
		return requestReviewResult{RunID: runID, Status: "rejected", Reason: err.Error()}, err
	}
	return requestReviewResult{RunID: runID, Status: "enqueued"}, nil
}

func (s *Server) isRunActive(runID string) bool {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	return s.active != nil && s.active[runID]
}

func (s *Server) markRunActive(runID string) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	if s.active == nil {
		s.active = make(map[string]bool)
	}
	s.active[runID] = true
}

func (s *Server) clearRunActive(runID string) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	delete(s.active, runID)
}

func requestRunID(req review.Request) string {
	project := strings.TrimSpace(firstNonEmptyString(req.ProjectID, req.Repository))
	change := strings.TrimSpace(firstNonEmptyString(req.ChangeID, strconv.Itoa(req.MRIID)))
	if project == "" || change == "" || change == "0" {
		return ""
	}
	return project + "!" + change
}
