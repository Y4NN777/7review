package tools

import (
	"context"
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
	approvedRun     string
	approvedReport  string
	publishedRun    string
	publishedReport string
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

type fakeReady struct{}

func (fakeReady) CheckReady(context.Context) (any, error) {
	return map[string]any{"ready": true}, nil
}

type fakeConfig struct{}

func (fakeConfig) ConfigStatus(context.Context) (any, error) {
	return map[string]any{"provider": "openrouter"}, nil
}

func TestToolExecutorExecutesReadOnlyTools(t *testing.T) {
	runs := &fakeRunTools{}
	executor := ToolExecutor{Runs: runs, Ready: fakeReady{}, Config: fakeConfig{}}

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

	for _, name := range []string{"check_ready", "get_config_status"} {
		if _, err := executor.Execute(context.Background(), ExecuteRequest{Name: name}); err != nil {
			t.Fatalf("%s failed: %v", name, err)
		}
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
