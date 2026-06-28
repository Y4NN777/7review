package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

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
