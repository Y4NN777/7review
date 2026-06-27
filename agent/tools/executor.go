package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ToolExecutor struct {
	Runs        RunReader
	Actions     RunActions
	Ready       ReadyChecker
	Config      ConfigStatusReader
	Skills      SkillLister
	Observatory Observatory
}

type RunReader interface {
	ListRuns(context.Context) (any, error)
	GetRun(context.Context, string) (any, error)
}

type RunActions interface {
	ApproveRun(context.Context, string, string) error
	PublishFinal(context.Context, string, string) error
	SuppressFinding(context.Context, string, string, string) error
}

type ReadyChecker interface {
	CheckReady(context.Context) (any, error)
}

type ConfigStatusReader interface {
	ConfigStatus(context.Context) (any, error)
}

type SkillLister interface {
	ListSkills(context.Context) (any, error)
}

type Observatory interface {
	SelectedContext(context.Context, string) (any, error)
	DiffSummary(context.Context, string) (any, error)
	ProviderStatus(context.Context) (any, error)
	PublishStatus(context.Context, string) (any, error)
	MemoryProposal(context.Context, string) (any, error)
}

type ExecuteRequest struct {
	Name  string         `json:"name"`
	Input map[string]any `json:"input,omitempty"`
}

type ExecuteResponse struct {
	Name   string `json:"name"`
	Result any    `json:"result,omitempty"`
}

func (e ToolExecutor) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ExecuteResponse{}, fmt.Errorf("tools: missing tool name")
	}
	if !catalogHasTool(name) {
		return ExecuteResponse{}, fmt.Errorf("tools: unknown tool %q", name)
	}

	switch name {
	case "list_runs":
		if e.Runs == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: run reader is not configured")
		}
		result, err := e.Runs.ListRuns(ctx)
		return ExecuteResponse{Name: name, Result: result}, err
	case "get_run":
		if e.Runs == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: run reader is not configured")
		}
		id := stringInput(req.Input, "id", "run")
		if id == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: get_run requires id")
		}
		result, err := e.Runs.GetRun(ctx, id)
		return ExecuteResponse{Name: name, Result: result}, err
	case "check_ready":
		if e.Ready == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: readiness checker is not configured")
		}
		result, err := e.Ready.CheckReady(ctx)
		return ExecuteResponse{Name: name, Result: result}, err
	case "get_config_status":
		if e.Config == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: config status reader is not configured")
		}
		result, err := e.Config.ConfigStatus(ctx)
		return ExecuteResponse{Name: name, Result: result}, err
	case "list_skills":
		if e.Skills == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: skill lister is not configured")
		}
		result, err := e.Skills.ListSkills(ctx)
		return ExecuteResponse{Name: name, Result: result}, err
	case "get_selected_context":
		if e.Observatory == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: observatory is not configured")
		}
		run := stringInput(req.Input, "run", "id")
		if run == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: get_selected_context requires run")
		}
		result, err := e.Observatory.SelectedContext(ctx, run)
		return ExecuteResponse{Name: name, Result: result}, err
	case "get_diff_summary":
		if e.Observatory == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: observatory is not configured")
		}
		run := stringInput(req.Input, "run", "id")
		if run == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: get_diff_summary requires run")
		}
		result, err := e.Observatory.DiffSummary(ctx, run)
		return ExecuteResponse{Name: name, Result: result}, err
	case "list_provider_status":
		if e.Observatory == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: observatory is not configured")
		}
		result, err := e.Observatory.ProviderStatus(ctx)
		return ExecuteResponse{Name: name, Result: result}, err
	case "get_publish_status":
		if e.Observatory == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: observatory is not configured")
		}
		run := stringInput(req.Input, "run", "id")
		if run == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: get_publish_status requires run")
		}
		result, err := e.Observatory.PublishStatus(ctx, run)
		return ExecuteResponse{Name: name, Result: result}, err
	case "preview_memory_proposal":
		if e.Observatory == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: observatory is not configured")
		}
		run := stringInput(req.Input, "run", "id")
		if run == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: preview_memory_proposal requires run")
		}
		result, err := e.Observatory.MemoryProposal(ctx, run)
		return ExecuteResponse{Name: name, Result: result}, err
	case "approve_run":
		if e.Actions == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: run actions are not configured")
		}
		run := stringInput(req.Input, "run")
		report := stringInput(req.Input, "report")
		if run == "" {
			run = legacyRunID(req.Input)
		}
		if run == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: approve_run requires run or project/mr")
		}
		if err := e.Actions.ApproveRun(ctx, run, report); err != nil {
			return ExecuteResponse{}, err
		}
		return ExecuteResponse{Name: name, Result: map[string]any{"accepted": true, "run": run}}, nil
	case "publish_final":
		if e.Actions == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: run actions are not configured")
		}
		run := stringInput(req.Input, "run")
		if run == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: publish_final requires run")
		}
		report := stringInput(req.Input, "report")
		if err := e.Actions.PublishFinal(ctx, run, report); err != nil {
			return ExecuteResponse{}, err
		}
		return ExecuteResponse{Name: name, Result: map[string]any{"accepted": true, "run": run}}, nil
	case "suppress_finding":
		if e.Actions == nil {
			return ExecuteResponse{}, fmt.Errorf("tools: run actions are not configured")
		}
		run := stringInput(req.Input, "run")
		findingID := stringInput(req.Input, "finding_id")
		reason := stringInput(req.Input, "reason")
		if run == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: suppress_finding requires run")
		}
		if findingID == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: suppress_finding requires finding_id")
		}
		if reason == "" {
			return ExecuteResponse{}, fmt.Errorf("tools: suppress_finding requires reason")
		}
		if err := e.Actions.SuppressFinding(ctx, run, findingID, reason); err != nil {
			return ExecuteResponse{}, err
		}
		return ExecuteResponse{Name: name, Result: map[string]any{"accepted": true, "run": run, "finding_id": findingID}}, nil
	case "stream_run_chat":
		return ExecuteResponse{}, fmt.Errorf("tools: stream_run_chat is executed through POST /chat/stream?run=<id>")
	default:
		return ExecuteResponse{}, fmt.Errorf("tools: %s is cataloged but not implemented", name)
	}
}

func catalogHasTool(name string) bool {
	for _, tool := range Catalog() {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func stringInput(input map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := input[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case json.Number:
			return v.String()
		case float64:
			return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.0f", v), ".0"), ".")
		}
	}
	return ""
}

func legacyRunID(input map[string]any) string {
	project := stringInput(input, "project")
	mr := stringInput(input, "mr")
	if project == "" || mr == "" {
		return ""
	}
	return project + "!" + mr
}
