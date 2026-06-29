package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/tools"
)

const toolExecuteMaxBodyBytes int64 = 64 * 1024

type appToolRunner struct {
	server *Server
}

func (r appToolRunner) ListRuns(ctx context.Context) (any, error) {
	runs, err := r.server.pipeline.Jobs.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runDTO, 0, len(runs))
	for _, run := range runs {
		out = append(out, toRunDTO(run, false))
	}
	return out, nil
}

func (r appToolRunner) GetRun(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toRunDTO(*run, true), nil
}

func (r appToolRunner) ApproveRun(ctx context.Context, run string, report string) error {
	return r.server.pipeline.ApproveRun(ctx, run, report)
}

func (r appToolRunner) PublishFinal(ctx context.Context, run string, report string) error {
	return r.server.pipeline.PublishFinal(ctx, run, report)
}

func (r appToolRunner) SuppressFinding(ctx context.Context, run string, findingID string, reason string) error {
	return r.server.pipeline.SuppressFinding(ctx, run, findingID, reason)
}

func (r appToolRunner) ReviseDraft(ctx context.Context, run string, request string) error {
	return r.server.pipeline.ReviseDraft(ctx, run, request)
}

func (r appToolRunner) RerunReview(ctx context.Context, run string, reason string) error {
	return r.server.pipeline.RerunReview(ctx, run, reason)
}

func (r appToolRunner) RequestReview(ctx context.Context, input tools.ReviewRequestInput) (any, error) {
	req := review.Request{Provider: input.Provider, EventAction: "manual"}
	switch input.Provider {
	case "gitlab":
		req.ProjectID = input.ProjectID
		req.MRIID = input.MRIID
		req.ChangeID = strconv.Itoa(input.MRIID)
	case "github":
		req.ProjectID = input.Repository
		req.Repository = input.Repository
		req.MRIID = input.PRNumber
		req.ChangeID = strconv.Itoa(input.PRNumber)
	}
	return r.server.requestReview(ctx, req)
}

func (r appToolRunner) CheckReady(ctx context.Context) (any, error) {
	return r.server.readiness(ctx), nil
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tools.Catalog())
}

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s == nil || s.pipeline == nil || s.pipeline.Jobs == nil {
		http.Error(w, "pipeline is not configured", http.StatusServiceUnavailable)
		return
	}
	body, err := readBoundedBody(r.Body, toolExecuteMaxBodyBytes)
	if err != nil {
		http.Error(w, "tool request too large", http.StatusRequestEntityTooLarge)
		return
	}
	var req tools.ExecuteRequest
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid tool request", http.StatusBadRequest)
		return
	}
	resp, err := (tools.ToolExecutor{
		Runs:        appToolRunner{server: s},
		Actions:     appToolRunner{server: s},
		Ready:       appToolRunner{server: s},
		Config:      appToolRunner{server: s},
		Skills:      appToolRunner{server: s},
		Observatory: appToolRunner{server: s},
	}).Execute(r.Context(), req)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not configured") {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

var _ tools.RunReader = appToolRunner{}
var _ tools.RunActions = appToolRunner{}
var _ tools.ReadyChecker = appToolRunner{}
