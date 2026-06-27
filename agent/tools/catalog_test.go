package tools

import (
	"strings"
	"testing"
)

func TestCatalogContainsModelFacingReviewTools(t *testing.T) {
	catalog := Catalog()
	want := map[string]bool{
		"list_runs":               false,
		"get_run":                 false,
		"stream_run_chat":         false,
		"check_ready":             false,
		"get_config_status":       false,
		"approve_run":             false,
		"publish_final":           false,
		"list_skills":             false,
		"get_selected_context":    false,
		"get_diff_summary":        false,
		"list_provider_status":    false,
		"get_publish_status":      false,
		"revise_draft":            false,
		"suppress_finding":        false,
		"rerun_review":            false,
		"preview_memory_proposal": false,
	}
	for _, tool := range catalog {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
		if tool.Description == "" || tool.InputSchema["type"] != "object" {
			t.Fatalf("invalid tool spec: %#v", tool)
		}
		if tool.LifecycleStage == "" {
			t.Fatalf("tool missing lifecycle stage: %#v", tool)
		}
		if tool.Implemented && tool.Executor == "" {
			t.Fatalf("implemented tool must declare executor: %#v", tool)
		}
		if tool.RequiresApproval && !tool.SideEffects {
			t.Fatalf("approval-gated tool must declare side effects: %#v", tool)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("missing tool %s in catalog %#v", name, catalog)
		}
	}
}

func TestCatalogMarksHILSideEffects(t *testing.T) {
	catalog := Catalog()
	byName := make(map[string]ToolSpec)
	for _, tool := range catalog {
		byName[tool.Name] = tool
	}
	for _, name := range []string{"approve_run", "publish_final"} {
		tool := byName[name]
		if !tool.SideEffects || !tool.RequiresApproval {
			t.Fatalf("%s must be side-effecting and approval-gated: %#v", name, tool)
		}
	}
	for _, name := range []string{"list_runs", "get_run", "stream_run_chat", "check_ready", "get_config_status", "list_skills", "get_selected_context", "get_diff_summary", "list_provider_status", "get_publish_status", "preview_memory_proposal"} {
		tool := byName[name]
		if tool.SideEffects || tool.RequiresApproval {
			t.Fatalf("%s should be read-only/non-gated: %#v", name, tool)
		}
	}
}

func TestCatalogImplementationStatusIsExplicit(t *testing.T) {
	catalog := Catalog()
	byName := make(map[string]ToolSpec)
	for _, tool := range catalog {
		byName[tool.Name] = tool
	}
	for _, name := range []string{"list_runs", "get_run", "stream_run_chat", "check_ready", "get_config_status", "approve_run", "publish_final"} {
		if !byName[name].Implemented {
			t.Fatalf("%s should be marked implemented: %#v", name, byName[name])
		}
	}
	for _, name := range []string{"list_skills", "get_selected_context", "get_diff_summary", "list_provider_status", "get_publish_status", "revise_draft", "suppress_finding", "rerun_review", "preview_memory_proposal"} {
		if byName[name].Implemented {
			t.Fatalf("%s should not be marked implemented until executor exists: %#v", name, byName[name])
		}
	}
}

func TestCatalogUsesProviderNeutralLanguageForRunIDsAndDependencies(t *testing.T) {
	catalog := Catalog()
	for _, tool := range catalog {
		if strings.Contains(tool.Description, "GitLab") || strings.Contains(tool.Description, "Headroom") || strings.Contains(tool.Description, "MemPalace") {
			t.Fatalf("tool description should be provider-neutral: %#v", tool)
		}
		if tool.Name == "get_run" {
			idSchema, ok := tool.InputSchema["properties"].(map[string]any)["id"].(map[string]any)
			if !ok {
				t.Fatalf("get_run id schema missing: %#v", tool.InputSchema)
			}
			description, _ := idSchema["description"].(string)
			if !strings.Contains(description, "Provider-neutral") {
				t.Fatalf("get_run id description should be provider-neutral: %q", description)
			}
		}
	}
}
