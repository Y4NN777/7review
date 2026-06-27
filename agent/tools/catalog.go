package tools

import "sort"

type ToolSpec struct {
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	LifecycleStage   string         `json:"lifecycle_stage"`
	Implemented      bool           `json:"implemented"`
	Executor         string         `json:"executor,omitempty"`
	SideEffects      bool           `json:"side_effects"`
	RequiresApproval bool           `json:"requires_approval"`
	InputSchema      map[string]any `json:"input_schema"`
}

func Catalog() []ToolSpec {
	tools := []ToolSpec{
		{
			Name:           "list_runs",
			Description:    "List review runs currently known by the agent.",
			LifecycleStage: "observe",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"list_runs\"}; also available as GET /runs",
			SideEffects:    false,
			InputSchema:    objectSchema(map[string]any{}),
		},
		{
			Name:           "get_run",
			Description:    "Fetch one review run with provider, status, draft report, final report, findings, and SCM URL.",
			LifecycleStage: "observe",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"get_run\"}; also available as GET /run?id=<id>",
			SideEffects:    false,
			InputSchema: objectSchema(map[string]any{
				"id": stringSchema("Provider-neutral run ID, usually <project-or-repository>!<change-id>."),
			}),
		},
		{
			Name:           "stream_run_chat",
			Description:    "Stream model-driven chat for one review run using stored findings and draft context.",
			LifecycleStage: "iterate",
			Implemented:    true,
			Executor:       "POST /chat/stream?run=<id>",
			SideEffects:    false,
			InputSchema: objectSchema(map[string]any{
				"run":     stringSchema("Run ID to discuss."),
				"message": stringSchema("Engineer message or question."),
			}),
		},
		{
			Name:           "check_ready",
			Description:    "Check required runtime dependencies and queue status: context compression, durable memory, run store, worker queue depth, and worker failures.",
			LifecycleStage: "preflight",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"check_ready\"}; also available as GET /ready",
			SideEffects:    false,
			InputSchema:    objectSchema(map[string]any{}),
		},
		{
			Name:           "get_config_status",
			Description:    "Inspect operator-visible configuration status: SCM provider, model provider, sidecar URLs, corpus root, memory path, and missing required settings without exposing secrets.",
			LifecycleStage: "preflight",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"get_config_status\"}",
			SideEffects:    false,
			InputSchema:    objectSchema(map[string]any{}),
		},
		{
			Name:           "list_skills",
			Description:    "List loaded skills with names, descriptions, activation status, and source paths for the current runtime.",
			LifecycleStage: "context",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"list_skills\"}",
			SideEffects:    false,
			InputSchema:    objectSchema(map[string]any{}),
		},
		{
			Name:           "get_selected_context",
			Description:    "Fetch selected PRD/SRS/contracts/rules/memory/context sections for one run, including citations and selection reasons.",
			LifecycleStage: "context",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"get_selected_context\"}",
			SideEffects:    false,
			InputSchema: objectSchema(map[string]any{
				"run": stringSchema("Run ID whose selected context should be inspected."),
			}),
		},
		{
			Name:           "get_diff_summary",
			Description:    "Fetch normalized changed files, patch chunk summaries, skipped/generated file notes, and token estimates for one run.",
			LifecycleStage: "diff",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"get_diff_summary\"}",
			SideEffects:    false,
			InputSchema: objectSchema(map[string]any{
				"run": stringSchema("Run ID whose normalized diff should be inspected."),
			}),
		},
		{
			Name:           "list_provider_status",
			Description:    "Show configured model providers, active model role chains, registered providers, and unavailable provider reasons without exposing secrets.",
			LifecycleStage: "preflight",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"list_provider_status\"}",
			SideEffects:    false,
			InputSchema:    objectSchema(map[string]any{}),
		},
		{
			Name:           "get_publish_status",
			Description:    "Inspect draft/final publish state, provider marker IDs or URLs, retry status, and HIL state for one run.",
			LifecycleStage: "publish",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"get_publish_status\"}",
			SideEffects:    false,
			InputSchema: objectSchema(map[string]any{
				"run": stringSchema("Run ID whose publish state should be inspected."),
			}),
		},
		{
			Name:           "revise_draft",
			Description:    "Apply an explicit engineer-requested draft report revision without approving final output or writing memory.",
			LifecycleStage: "iterate",
			Implemented:    false,
			SideEffects:    true,
			InputSchema: objectSchema(map[string]any{
				"run":     stringSchema("Run ID whose draft should be revised."),
				"request": stringSchema("Engineer revision request."),
			}),
		},
		{
			Name:             "suppress_finding",
			Description:      "Suppress or reject one finding with an explicit engineer reason so it is excluded from final output and memory.",
			LifecycleStage:   "iterate",
			Implemented:      true,
			Executor:         "POST /tools/execute {\"name\":\"suppress_finding\"}",
			SideEffects:      true,
			RequiresApproval: true,
			InputSchema: objectSchema(map[string]any{
				"run":        stringSchema("Run ID containing the finding."),
				"finding_id": stringSchema("Finding ID to suppress."),
				"reason":     stringSchema("Human-readable suppression reason."),
			}),
		},
		{
			Name:           "rerun_review",
			Description:    "Rerun review for one change after new commits, updated context, or explicit engineer request.",
			LifecycleStage: "review",
			Implemented:    false,
			SideEffects:    true,
			InputSchema: objectSchema(map[string]any{
				"run":    stringSchema("Existing run ID to rerun from."),
				"reason": stringSchema("Why the review should be rerun."),
			}),
		},
		{
			Name:           "preview_memory_proposal",
			Description:    "Preview what approved final review knowledge would write to durable memory before the write occurs.",
			LifecycleStage: "memory",
			Implemented:    true,
			Executor:       "POST /tools/execute {\"name\":\"preview_memory_proposal\"}",
			SideEffects:    false,
			InputSchema: objectSchema(map[string]any{
				"run": stringSchema("Run ID whose memory proposal should be previewed."),
			}),
		},
		{
			Name:             "approve_run",
			Description:      "Continue after human approval. This is a HIL action and must not be inferred.",
			LifecycleStage:   "hil",
			Implemented:      true,
			Executor:         "POST /tools/execute {\"name\":\"approve_run\"}; also available as POST /approve?run=<id>",
			SideEffects:      true,
			RequiresApproval: true,
			InputSchema: objectSchema(map[string]any{
				"run":     stringSchema("Run ID to approve. Preferred over project/mr."),
				"project": stringSchema("Legacy SCM project or repository identifier."),
				"mr":      numberSchema("Legacy merge request or pull request IID/number."),
				"report":  stringSchema("Approved final report content."),
			}),
		},
		{
			Name:             "publish_final",
			Description:      "Publish final review output to the SCM provider after approval.",
			LifecycleStage:   "publish",
			Implemented:      true,
			Executor:         "POST /tools/execute {\"name\":\"publish_final\"}; also available as POST /publish/final?run=<id>",
			SideEffects:      true,
			RequiresApproval: true,
			InputSchema: objectSchema(map[string]any{
				"run":    stringSchema("Run ID to publish."),
				"report": stringSchema("Final report content."),
			}),
		},
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

func objectSchema(properties map[string]any) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": properties,
	}
}

func stringSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func numberSchema(description string) map[string]any {
	return map[string]any{
		"type":        "number",
		"description": description,
	}
}
