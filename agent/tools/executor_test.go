package tools

import (
	"context"
	"strings"
	"testing"
)

type fakeRunTools struct {
	listed bool
	gotID  string
}

func (f *fakeRunTools) ListRuns(context.Context) (any, error) {
	f.listed = true
	return []string{"p!1"}, nil
}

func (f *fakeRunTools) GetRun(_ context.Context, id string) (any, error) {
	f.gotID = id
	return map[string]any{"id": id}, nil
}

type fakeActions struct {
	approvedRun      string
	approvedReport   string
	publishedRun     string
	publishedReport  string
	suppressedRun    string
	suppressedID     string
	suppressedReason string
	revisedRun       string
	revisedRequest   string
	rerunRun         string
	rerunReason      string
	reviewInput      ReviewRequestInput
}

func (f *fakeActions) RequestReview(_ context.Context, input ReviewRequestInput) (any, error) {
	f.reviewInput = input
	return map[string]any{"run_id": input.ProjectID + "!7", "status": "enqueued"}, nil
}

func (f *fakeActions) ApproveRun(_ context.Context, run string, report string) error {
	f.approvedRun = run
	f.approvedReport = report
	return nil
}

func (f *fakeActions) PublishFinal(_ context.Context, run string, report string) error {
	f.publishedRun = run
	f.publishedReport = report
	return nil
}

func (f *fakeActions) SuppressFinding(_ context.Context, run string, findingID string, reason string) error {
	f.suppressedRun = run
	f.suppressedID = findingID
	f.suppressedReason = reason
	return nil
}

func (f *fakeActions) ReviseDraft(_ context.Context, run string, request string) error {
	f.revisedRun = run
	f.revisedRequest = request
	return nil
}

func (f *fakeActions) RerunReview(_ context.Context, run string, reason string) error {
	f.rerunRun = run
	f.rerunReason = reason
	return nil
}

type fakeReady struct{}

func (fakeReady) CheckReady(context.Context) (any, error) {
	return map[string]any{"ready": true}, nil
}

type fakeConfig struct{}

func (fakeConfig) ConfigStatus(context.Context) (any, error) {
	return map[string]any{"provider": "openrouter"}, nil
}

type fakeSkills struct{}

func (fakeSkills) ListSkills(context.Context) (any, error) {
	return []string{"methodology-review"}, nil
}

type fakeObservatory struct {
	selectedRun string
	timelineRun string
	diffRun     string
	mrRun       string
	filesRun    string
	inlineRun   string
	discussRun  string
	publishRun  string
	providerHit bool
}

func (f *fakeObservatory) SelectedContext(_ context.Context, run string) (any, error) {
	f.selectedRun = run
	return map[string]any{"run": run}, nil
}

func (f *fakeObservatory) RunTimeline(_ context.Context, run string) (any, error) {
	f.timelineRun = run
	return map[string]any{"run": run, "events": []string{"run_started"}}, nil
}

func (f *fakeObservatory) DiffSummary(_ context.Context, run string) (any, error) {
	f.diffRun = run
	return map[string]any{"run": run}, nil
}

func (f *fakeObservatory) MergeRequest(_ context.Context, run string) (any, error) {
	f.mrRun = run
	return map[string]any{"run": run}, nil
}

func (f *fakeObservatory) ChangedFiles(_ context.Context, run string) (any, error) {
	f.filesRun = run
	return map[string]any{"run": run}, nil
}

func (f *fakeObservatory) InlinePositions(_ context.Context, run string) (any, error) {
	f.inlineRun = run
	return map[string]any{"run": run}, nil
}

func (f *fakeObservatory) Discussions(_ context.Context, run string) (any, error) {
	f.discussRun = run
	return map[string]any{"run": run}, nil
}

func (f *fakeObservatory) ProviderStatus(context.Context) (any, error) {
	f.providerHit = true
	return map[string]any{"providers": []string{"openrouter"}}, nil
}

func (f *fakeObservatory) PublishStatus(_ context.Context, run string) (any, error) {
	f.publishRun = run
	return map[string]any{"run": run}, nil
}

func (f *fakeObservatory) MemoryProposal(_ context.Context, run string) (any, error) {
	return map[string]any{"run": run, "approved": true}, nil
}

func TestToolExecutorExecutesReadOnlyTools(t *testing.T) {
	runs := &fakeRunTools{}
	executor := ToolExecutor{Runs: runs, Ready: fakeReady{}, Config: fakeConfig{}, Skills: fakeSkills{}}

	if _, err := executor.Execute(context.Background(), ExecuteRequest{Name: "list_runs"}); err != nil {
		t.Fatal(err)
	}
	if !runs.listed {
		t.Fatal("list_runs did not call run reader")
	}

	if _, err := executor.Execute(context.Background(), ExecuteRequest{Name: "get_run", Input: map[string]any{"id": "owner/repo!7"}}); err != nil {
		t.Fatal(err)
	}
	if runs.gotID != "owner/repo!7" {
		t.Fatalf("unexpected get_run id %q", runs.gotID)
	}

	for _, name := range []string{"check_ready", "get_config_status", "list_skills"} {
		if _, err := executor.Execute(context.Background(), ExecuteRequest{Name: name}); err != nil {
			t.Fatalf("%s failed: %v", name, err)
		}
	}
}

func TestToolExecutorExecutesObservabilityTools(t *testing.T) {
	observatory := &fakeObservatory{}
	executor := ToolExecutor{Observatory: observatory}

	for _, req := range []ExecuteRequest{
		{Name: "get_selected_context", Input: map[string]any{"run": "p!7"}},
		{Name: "get_run_timeline", Input: map[string]any{"id": "p!7"}},
		{Name: "get_diff_summary", Input: map[string]any{"id": "p!7"}},
		{Name: "get_merge_request", Input: map[string]any{"id": "p!7"}},
		{Name: "get_changed_files", Input: map[string]any{"id": "p!7"}},
		{Name: "get_inline_positions", Input: map[string]any{"id": "p!7"}},
		{Name: "list_discussions", Input: map[string]any{"id": "p!7"}},
		{Name: "list_provider_status"},
		{Name: "get_publish_status", Input: map[string]any{"run": "p!7"}},
		{Name: "preview_memory_proposal", Input: map[string]any{"run": "p!7"}},
	} {
		if _, err := executor.Execute(context.Background(), req); err != nil {
			t.Fatalf("%s failed: %v", req.Name, err)
		}
	}
	if observatory.selectedRun != "p!7" || observatory.timelineRun != "p!7" || observatory.diffRun != "p!7" ||
		observatory.mrRun != "p!7" || observatory.filesRun != "p!7" || observatory.discussRun != "p!7" ||
		observatory.publishRun != "p!7" || !observatory.providerHit {
		t.Fatalf("observability tools were not called correctly: %#v", observatory)
	}
}

func TestToolExecutorExecutesApprovalAndPublish(t *testing.T) {
	actions := &fakeActions{}
	executor := ToolExecutor{Actions: actions}

	if _, err := executor.Execute(context.Background(), ExecuteRequest{
		Name:  "approve_run",
		Input: map[string]any{"project": "p", "mr": float64(7), "report": "final"},
	}); err != nil {
		t.Fatal(err)
	}
	if actions.approvedRun != "p!7" || actions.approvedReport != "final" {
		t.Fatalf("unexpected approval call: %#v", actions)
	}

	if _, err := executor.Execute(context.Background(), ExecuteRequest{
		Name:  "publish_final",
		Input: map[string]any{"run": "p!7", "report": "final"},
	}); err != nil {
		t.Fatal(err)
	}
	if actions.publishedRun != "p!7" || actions.publishedReport != "final" {
		t.Fatalf("unexpected publish call: %#v", actions)
	}

	if _, err := executor.Execute(context.Background(), ExecuteRequest{
		Name:  "suppress_finding",
		Input: map[string]any{"run": "p!7", "finding_id": "F1", "reason": "false positive"},
	}); err != nil {
		t.Fatal(err)
	}
	if actions.suppressedRun != "p!7" || actions.suppressedID != "F1" || actions.suppressedReason != "false positive" {
		t.Fatalf("unexpected suppress call: %#v", actions)
	}

	if _, err := executor.Execute(context.Background(), ExecuteRequest{
		Name:  "revise_draft",
		Input: map[string]any{"run": "p!7", "request": "clarify evidence"},
	}); err != nil {
		t.Fatal(err)
	}
	if actions.revisedRun != "p!7" || actions.revisedRequest != "clarify evidence" {
		t.Fatalf("unexpected revise call: %#v", actions)
	}

	if _, err := executor.Execute(context.Background(), ExecuteRequest{
		Name:  "rerun_review",
		Input: map[string]any{"run": "p!7", "reason": "new commits"},
	}); err != nil {
		t.Fatal(err)
	}
	if actions.rerunRun != "p!7" || actions.rerunReason != "new commits" {
		t.Fatalf("unexpected rerun call: %#v", actions)
	}
}

func TestToolExecutorExecutesRequestReview(t *testing.T) {
	actions := &fakeActions{}
	executor := ToolExecutor{Actions: actions}

	resp, err := executor.Execute(context.Background(), ExecuteRequest{
		Name:  "request_review",
		Input: map[string]any{"provider": "github", "repository": "owner/repo", "pr_number": float64(7)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if actions.reviewInput.Provider != "github" || actions.reviewInput.Repository != "owner/repo" || actions.reviewInput.PRNumber != 7 {
		t.Fatalf("unexpected request review input: %#v", actions.reviewInput)
	}
	if resp.Result == nil {
		t.Fatalf("expected request review result")
	}
}

func TestImplementedCatalogToolsHaveExecutablePath(t *testing.T) {
	executor := ToolExecutor{
		Runs:        &fakeRunTools{},
		Actions:     &fakeActions{},
		Ready:       fakeReady{},
		Config:      fakeConfig{},
		Skills:      fakeSkills{},
		Observatory: &fakeObservatory{},
	}
	inputs := map[string]map[string]any{
		"get_run":                 {"id": "p!7"},
		"get_run_timeline":        {"run": "p!7"},
		"stream_run_chat":         {"run": "p!7", "message": "hello"},
		"get_selected_context":    {"run": "p!7"},
		"get_diff_summary":        {"run": "p!7"},
		"get_merge_request":       {"run": "p!7"},
		"get_changed_files":       {"run": "p!7"},
		"get_inline_positions":    {"run": "p!7"},
		"list_discussions":        {"run": "p!7"},
		"get_publish_status":      {"run": "p!7"},
		"preview_memory_proposal": {"run": "p!7"},
		"revise_draft":            {"run": "p!7", "request": "clarify evidence"},
		"request_review":          {"provider": "github", "repository": "owner/repo", "pr_number": 7},
		"suppress_finding":        {"run": "p!7", "finding_id": "F1", "reason": "false positive"},
		"rerun_review":            {"run": "p!7", "reason": "new commits"},
		"approve_run":             {"run": "p!7", "report": "final"},
		"publish_final":           {"run": "p!7", "report": "final"},
	}
	for _, tool := range Catalog() {
		if !tool.Implemented {
			continue
		}
		_, err := executor.Execute(context.Background(), ExecuteRequest{Name: tool.Name, Input: inputs[tool.Name]})
		if tool.Name == "stream_run_chat" {
			if err == nil || !strings.Contains(err.Error(), "/chat/stream") {
				t.Fatalf("stream_run_chat should route to streaming endpoint guidance, got %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s is marked implemented but not executable: %v", tool.Name, err)
		}
	}
}

func TestToolExecutorRejectsUnknownAndUnimplementedTools(t *testing.T) {
	executor := ToolExecutor{}
	if _, err := executor.Execute(context.Background(), ExecuteRequest{Name: "missing_tool"}); err == nil {
		t.Fatal("expected unknown tool error")
	}
	if _, err := executor.Execute(context.Background(), ExecuteRequest{Name: "get_selected_context"}); err == nil {
		t.Fatal("expected unimplemented tool error")
	}
}

func TestToolExecutorDocumentsStreamingChatEndpoint(t *testing.T) {
	executor := ToolExecutor{}
	_, err := executor.Execute(context.Background(), ExecuteRequest{Name: "stream_run_chat", Input: map[string]any{"run": "p!7", "message": "hello"}})
	if err == nil {
		t.Fatal("expected stream endpoint guidance")
	}
}
