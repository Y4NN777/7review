package app

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/Y4NN777/7review/agent/review"
)

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
