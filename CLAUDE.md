# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

7review is a Go code-review agent for GitHub pull requests and GitLab merge requests. It receives SCM
webhooks, enriches the change with provider metadata, selects repository knowledge and skills, runs
model review, validates findings deterministically, publishes a draft report, waits for human approval,
then publishes the final report and writes approved memory. Module path: `github.com/Y4NN777/7review`.

## Commands

```sh
gofmt -w ./cmd/7review ./agent/...      # format (required before commit)
go test ./...                            # run all tests
make verify                              # fmt + bridge-check + test + docker-config (full local gate)
go run ./cmd/7review setup               # generate local .env
go run ./cmd/7review                     # run the server (needs .env sourced)
go run ./cmd/7review status --server http://localhost:8080
```

Run a single test or package:

```sh
go test ./agent/pipeline -run TestSelectCorpus -v
go test ./agent/pipeline -run 'TestSelectCorpus|TestCorpusGraph|TestExtractReviewSignals' -v
```

Live model smoke test (calls a real local Ollama model, not part of the default suite):

```sh
RUN_LIVE_SMOKE=1 OLLAMA_BASE_URL=http://127.0.0.1:11434 ORCHESTRATOR_CONFIG=./orchestrator.yaml \
  go test -tags live_smoke ./agent/pipeline -run TestLiveSmokeReviewPipelineWithConfiguredOllamaModels -count=1 -v
```

Docker runtime (agent + Headroom bridge + MemPalace bridge):

```sh
make docker-config   # validate compose config
make docker-up
make docker-status
make docker-logs
make bridge-check    # py_compile + tests for the Python sidecar bridges
```

Manually enqueue a review (same authenticated path as the webhook queue):

```sh
go run ./cmd/7review review gitlab --project <id> --mr <iid> --server http://localhost:8080
go run ./cmd/7review review github --repo <owner/repo> --pr <number> --server http://localhost:8080
```

## Architecture

The system splits into two planes: the **review plane** (webhook intake → SCM enrichment → context
selection → model review → finding validation → draft publish → HIL approval → final publish → memory
write) and the **operator plane** (authenticated tools, run inspection, chat, CLI, TUI).

Package responsibilities — keep new code aligned to these boundaries:

- `cmd/7review`: CLI entrypoint, `setup`/`status` commands, operator client/DTOs, Bubble Tea TUI.
- `agent/app`: HTTP routes, auth, webhook handlers, the bounded worker queue, chat streaming, readiness.
  Webhook handlers never run review work inline — they verify the signature, normalize into
  `review.Request`, apply webhook policy, dedupe by delivery ID, and enqueue a `workItem`.
- `agent/pipeline`: the review lifecycle (`Pipeline.Run`), the `RunStore`, corpus graph
  selection/retrieval, `FindingValidator`, report rendering, HIL/final-publish gates.
- `agent/review`: provider-neutral domain model — `Request`, `Source`, `StructuredDiff`, `EvidenceItem`,
  `Finding`, report/run state. No SCM or model-provider logic belongs here.
- `agent/tools`: GitHub/GitLab adapters (`SCM`/`Publisher` interfaces), the `ProviderRouter`, Headroom
  and MemPalace sidecar clients, the operator tool catalog and executor.
- `agent/orchestrator`: semantic model-role routing (`reasoner`, `formatter`, `embedder`), provider
  fallback chains, streaming, and parallel reasoner fan-out (`CompleteParallel`).
- `agent/llm/providers`: concrete provider clients (OpenAI, Anthropic, OpenRouter, DeepSeek, Mistral,
  Gemini, Ollama, OpenAI-compatible).
- `agent/skills`: loads/validates `agent/skills/<skill-name>/SKILL.md` (YAML frontmatter + Markdown body;
  directory name must match frontmatter `name`).
- `agent/ui`: Lip Gloss-based renderers shared by setup, status, and chat surfaces.

Full component diagrams, the review lifecycle sequence, the state machine, and the corpus graph
retrieval design are in `docs/architecture.md` — read it before making non-trivial pipeline or context-
selection changes. Current implementation state, known review-quality limits, and the finding-strength
gate design are in `docs/status.md`.

### Review lifecycle (high level)

`Pipeline.Run`: start run → enrich via `tools.SCM` → normalize diff → select skills → select repo
knowledge via the corpus graph → recall MemPalace memory → apply `PolicyFilter` → reduce context via
Headroom → run the reasoner (`CompleteParallel` over diff batches) → parse/validate JSON findings →
render + publish draft → mark run `drafted`. Final publication and memory writeback are separate,
human-gated actions (`ApproveRun` / `PublishFinal`) — chat and model output can never approve a run.

### Finding validation is the trust boundary

`DefaultFindingValidator` (`agent/pipeline`) classifies every model-proposed issue into
confirmed/likely/speculative/note/question and rejects duplicates, invalid severities, low-confidence
findings, and anything outside a changed-file location. Only `confirmed` findings with verifiable
source-of-truth citations become inline draft comments; everything else stays in draft-only sections.
Do not weaken this gate to make findings "look better" — it exists specifically to keep model output
from being treated as autonomous judgment (see `docs/status.md`).

### Config and required env

Production requires one SCM provider (GitHub or GitLab), one model provider/endpoint, `HEADROOM_URL`,
`MEMPALACE_URL`, and `REVIEW_API_TOKEN`. `agent/config` loads and validates env; `orchestrator.yaml`
(optional) configures multi-provider role routing and must stay secret-free — see `.env.example` and
`AGENTS.md` for the full variable list. Configured production adapters are strict: if config names a
provider but the pipeline would fall back to a no-op adapter, `Pipeline.Run` rejects the run.

## Conventions

- Standard `gofmt` formatting; exported names are descriptive (`BuildOrchestrator`, `review.Context`),
  unexported helpers are lower camelCase. One provider per file; keep orchestration, config loading, and
  HTTP wiring in separate files.
- Tests live beside the code as `*_test.go`, named `TestFunctionName_Behavior`. Avoid real external API
  calls — use fake `LLMProvider`/`SCM`/`Publisher` implementations or local `httptest` servers (see
  `agent/app/test_fakes_test.go` for existing fakes).
- Webhook processing must stay bounded — never reintroduce unbounded fire-and-forget goroutines in
  request handlers; use the existing `WEBHOOK_WORKERS`/`WEBHOOK_QUEUE_SIZE` worker pool.
- Headroom and MemPalace are external HTTP sidecars (Python bridges under `docker/`), not embedded logic
  in the Go binary — reach them only through `HEADROOM_URL`/`MEMPALACE_URL` clients in `agent/tools`.
- Do not commit real tokens/API keys; copy `.env.example` to a local ignored `.env`.
