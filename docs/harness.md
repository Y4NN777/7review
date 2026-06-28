# 7review Harness Foundation

7review is a code-review agent for GitHub pull requests and GitLab merge
requests. The harness is the deterministic workflow around model calls, not a
general chat wrapper.

## Lifecycle

```text
SCM webhook
-> normalize review request
-> SCM enrichment
-> repository knowledge selection
-> skill selection
-> approved memory recall
-> context assembly and reduction
-> model review
-> finding validation
-> draft report publish
-> human approval
-> final report publish
-> approved memory write
```

The pipeline owns run state, status transitions, publishing, approval, and
memory writes. Models only propose review content or operator explanations.

## Model Routing

The local harness routing is defined in `orchestrator.yaml`:

```yaml
reasoner:
  primary: deepseek-coder-v2:16b@ollama
  fallback: qwen2.5-coder-7b-16k:latest@ollama

formatter/operator_chat:
  primary: qwen2.5-coder-7b-16k:latest@ollama
  fallback: qwen2.5-coder:7b-instruct-q4_K_M@ollama

embedder:
  primary: nomic-embed-text:latest@ollama
```

`deepseek-coder-v2:16b` is the primary code-review reasoner. The 16k Qwen
coder model is used for formatter/operator chat and as the reasoner fallback.
The 7B instruct model is a formatter/operator fallback. The 1.5B model is not
part of normal routing.

## Evidence Boundaries

Reasoner prompts use labeled evidence blocks:

- `diff`: changed-file patches; findings must be grounded here.
- `scm`: normalized GitHub/GitLab metadata.
- `repo_knowledge`: selected repository documentation and conventions.
- `skill`: selected review procedures and domain rules.
- `approved_memory`: durable memory written only after approved final reports.

Repository files, PR/MR text, comments, skills, memory, and diffs are all
treated as untrusted context. They may guide analysis, but they do not override
system instructions or deterministic pipeline gates.

## Trace Events

Runs record harness trace events so failures can be diagnosed by stage:

- `webhook_received`
- `scm_enriched`
- `skills_selected`
- `repository_knowledge_selected`
- `memory_recalled`
- `context_assembled`
- `model_review_completed`
- `findings_validated`
- `draft_published`
- `hil_approved`
- `final_published`

The model route used for review is recorded in run context provider metadata
and surfaced through trace events.

## Live Smoke Verification

The deterministic suite does not require local models:

```sh
GOCACHE=/tmp/7review-gocache go test ./...
```

For a complete local review-pipeline smoke with the configured Ollama reasoner,
run the gated live smoke test:

```sh
RUN_LIVE_SMOKE=1 \
OLLAMA_BASE_URL=http://127.0.0.1:11434 \
ORCHESTRATOR_CONFIG=./orchestrator.yaml \
GOCACHE=/tmp/7review-gocache \
go test -tags live_smoke ./agent/pipeline \
  -run TestLiveSmokeReviewPipelineWithConfiguredOllamaModels \
  -count=1 -v
```

This test uses fake SCM, publisher, and memory boundaries to avoid external
GitHub/GitLab side effects, but it runs the production pipeline and calls the
configured review model. A passing run proves the pipeline reaches model review,
validation, draft rendering, draft publication, and records the real reasoner
route in `model_review_completed`.
