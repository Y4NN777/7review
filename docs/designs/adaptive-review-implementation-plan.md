# Implementation Plan: Adaptive Review Platform

Status: CLEARED FOR PHASE 0 IMPLEMENTATION
Depends on: `adaptive-review-platform.md`

## Delivery Rules

- Preserve behavior before changing composition.
- One architectural concern per commit.
- Keep `main` green after every phase.
- Do not add channels, deployment platforms, or executable policy engines.
- Run deterministic tests before model/network evaluations.
- Persist compatibility evidence and migration notes in each phase.

## Phase 0: Characterize And Freeze

Goal: establish a reproducible baseline for the current primitives.

- Add golden lifecycle fixtures for GitHub and GitLab normalized requests.
- Capture current skill activations, corpus selections, validator outcomes,
  publication gates, run events, and memory boundaries.
- Add a behavior-equivalence helper that compares normalized artifacts rather
  than report prose.
- Define package dependency and run-state diagrams in architecture docs.

Exit: deterministic baseline scenarios pass on current code with no runtime
behavior change.

## Phase 1: Canonical Run Source

Goal: remove duplicated state before introducing new decisions.

- Add missing plan/repository identity placeholders to `review.Source`.
- Migrate pipeline, stores, Headroom, MemPalace, app DTOs, and tests to read and
  write canonical `Source` fields.
- Reduce `review.Context` to synchronization and transient batch accumulation.
- Add invariant tests that persisted and in-memory artifacts cannot diverge.

Exit: no request, diff, skill, finding, report, or metadata field has two
authoritative copies.

## Phase 2: Repository Snapshot Contract

Goal: bind all repository knowledge to an identified revision.

- Introduce snapshot identity and `fs.FS` contracts.
- Implement verified mounted-workspace snapshots for local/Docker operation.
- Extend GitHub/GitLab adapters with bounded base-revision file/tree access or
  snapshot materialization.
- Route corpus, repository skills, rules, and future policy through snapshots.
- Treat proposed head policy as evidence only.

Exit: a run fails planning when repository identity or trusted revision cannot
be established; path traversal and oversized content tests pass.

## Phase 3: Review Plan Compatibility Layer

Goal: make the effective strategy explicit without changing outcomes.

- Define immutable `review.Plan`, provenance, conflicts, classification, and
  fingerprint types.
- Implement `LegacyPlanCompiler` from `CompiledProfile`, current skill settings,
  corpus policy, validators, tools, and publishing defaults.
- Persist the plan before model execution and expose it through run status.
- Make pipeline selection consume legacy plan fields incrementally.

Exit: baseline scenarios produce behavior-equivalent results plus a stable plan.

## Phase 4: Declarative Repository Policy

Goal: let repositories define scoped review behavior safely.

- Add versioned YAML structs and JSON schemas for project config, packs, rules,
  and scenario manifests.
- Implement strict parsing, reference validation, namespace rules, inheritance
  cycle detection, path confinement, and bounded values.
- Implement deterministic classification and resolver merge laws.
- Add `7review validate-policy` and `7review explain`.
- Review policy changes using trusted base policy.

Exit: unit fixtures cover every predicate, merge rule, conflict, provenance
path, unsafe override, and fingerprint stability case.

## Phase 5: Staged Engine Extraction

Goal: split the monolithic pipeline along tested lifecycle boundaries.

- Introduce stage interfaces only where a deterministic or external boundary
  already exists.
- Extract planning, evidence gathering, model execution, validation, draft
  composition, approval, publication, and memory writeback in that order.
- Keep one engine responsible for transitions, retries, cancellation, and run
  persistence.
- Preserve existing public app and tool APIs during extraction.
- Replace the implicit tool follow-up loop with typed hypotheses, required-check
  coverage, investigation budgets, progress detection, and explicit stop reasons.
- Keep the controller single-agent. Add a selective verifier only after
  deterministic candidate validation and only for plan-selected risk classes.
- Persist trajectory artifacts without hidden model reasoning.

Exit: stage contract tests can exercise planning through final publication with
fakes; loop tests prove completion, no-progress, budget exhaustion, superseded
head, escalation, deduplication, and verifier boundaries; current app E2E tests
remain unchanged and green.

## Phase 6: Capability Registry

Goal: make tools auditable and prevent catalog/executor drift.

- Register descriptor and handler together.
- Add actor scope (`model`, `operator`, `system`), side-effect class, lifecycle
  stage, approval requirement, and plan capability checks.
- Migrate existing read tools first, then operator actions.
- Keep model and operator execution entrypoints separate.

Exit: duplicate names, missing handlers, schema drift, unauthorized actors, and
unapproved side effects fail deterministically.

## Phase 7: Scenario And Evaluation Harness

Goal: turn review strategy and quality into measurable product behavior.

- Implement scenario fixture loading, patch application, expected-plan checks,
  normalized finding scoring, repetition, and machine-readable reports.
- Add trajectory graders for hypothesis yield, evidence gain, coverage, stop
  reason, escalation correctness, and cost.
- Land at least 24 deterministic logical scenarios.
- Add a small tagged live-model subset using `openrouter/free` by default.
- Add budgets and statistical tolerance for non-deterministic models.

Exit: local and CI deterministic suites report strategy accuracy and lifecycle
correctness; opt-in live eval reports finding metrics without gating on prose.

## Phase 8: GitHub/GitLab Parity And Product Surface

Goal: prove the architecture against real SCM behavior and make it usable.

- Materialize curated scenarios as temporary GitHub PRs and GitLab MRs.
- Verify normalization, plans, draft/final markers, retry idempotency, HIL,
  memory writeback, and cleanup.
- Expose plan, provenance, conflicts, and scenario results in operator APIs,
  CLI/TUI, and documentation.
- Update Docker and CI only for new commands and fixtures, not new services.

Exit: equivalent logical scenarios produce equivalent plans across providers;
at least one complete controlled E2E passes on each provider.

## Atomic Commit Sequence

1. `test(review): characterize current lifecycle artifacts`
2. `refactor(review): make source the canonical run state`
3. `feat(repository): add trusted snapshot contract`
4. `feat(scm): load base snapshots through github and gitlab`
5. `feat(review): compile legacy behavior into review plans`
6. `feat(policy): add review pack schema and resolver`
7. `feat(cli): explain and validate repository policy`
8. `refactor(engine): extract explicit review stages`
9. `feat(engine): add bounded hypothesis review loop`
10. `refactor(tools): register governed capabilities`
11. `test(eval): add adaptive review scenario harness`
12. `test(scm): verify github and gitlab scenario parity`
13. `docs(review): publish adaptive review operating model`

## Verification Gates

```bash
GOCACHE=/tmp/7review-go-cache go test ./agent/review ./agent/profile
GOCACHE=/tmp/7review-go-cache go test ./agent/policy ./agent/repository
GOCACHE=/tmp/7review-go-cache go test ./agent/skills ./agent/tools
GOCACHE=/tmp/7review-go-cache go test ./agent/pipeline ./agent/app
GOCACHE=/tmp/7review-go-cache go test ./...
make verify
make compose-smoke
```

Live model and SCM E2E gates are explicit, credentialed jobs and do not replace
deterministic CI.

## First Implementation Slice

Start with Phase 0 and the smallest part of Phase 1:

1. Add canonical artifact comparison fixtures around one GitHub and one GitLab
   run.
2. Inventory every duplicated `Context`/`Source` field and define the migration
   order.
3. Move request and diff authority to `Source` first.
4. Keep compatibility accessors until pipeline and tests migrate.
5. Commit only after focused tests and `go test ./...` pass.

Do not introduce repository policy parsing in the first slice. The decision
engine must land on a coherent state model, not deepen the current duplication.

## GSTACK REVIEW REPORT

Review target: complete 7review system direction, including architecture,
quality, tests, performance, security, operations, and agent behavior.

Status: **APPROVED WITH PHASE GATES**

### Scope Decision

The policy engine alone is insufficient. The accepted scope is the adaptive
review platform: canonical run state, trusted repository snapshots, compiled
plans, staged execution, a bounded hypothesis loop, governed capabilities,
evaluation scenarios, and provider parity. Existing channels, Docker packaging,
model adapters, HIL, evidence graph, and memory are preserved and frozen unless
a compatibility change is required.

### Architecture Review

- Preserve the rich primitives; do not rewrite the product around YAML.
- Make `review.Source` authoritative before adding new policy state.
- Bind policy, rules, methods, and corpus to one trusted base snapshot.
- Keep one deterministic engine and one bounded review agent, not a swarm.
- Separate repository methodology from deployment/operator configuration.
- Keep model tools read-only and operator mutations on a distinct actor surface.

Primary production failure addressed: a run must not silently use policy or
corpus from the wrong repository/revision. Planning fails closed when snapshot
identity cannot be verified.

### Code Quality Review

- The largest risk is synchronization drift between duplicated `Context` and
  `Source` fields. Phase 1 removes it before new features.
- The 2,600-line pipeline and static tool dispatch are decomposition targets,
  but extraction follows behavior characterization and existing boundaries.
- New interfaces are allowed only for domain, deterministic, or external I/O
  boundaries already visible in the code.

### Test Review

- Existing package tests remain regression gates.
- New resolver tests are model-free and exact.
- Agent-loop tests grade trajectories, progress, stop reasons, and escalation.
- Real-model evals use normalized outcomes and repeated runs, not exact prose.
- GitHub/GitLab parity and provider E2E remain separate credentialed layers.

### Performance Review

- Snapshot reads, evidence bytes, tool calls, rounds, model calls, and elapsed
  time are budgeted and observable.
- Snapshots are cacheable by repository plus revision, immutable, and bounded.
- Parallel review workers share one plan and merge typed hypotheses before final
  validation to prevent duplicate comments and context multiplication.
- Headroom reduces context but is never a correctness dependency.

### Security Review

- Base-policy authority and snapshot identity are hard invariants.
- Repository policy remains non-executable and path-confined.
- The effective allowlist is policy permissions intersected with runtime
  permissions; repository policy can never grant a forbidden capability.
- Final publication and memory remain explicitly human-authorized.

### Readiness Dashboard

| Area | State | Gate before implementation |
| --- | --- | --- |
| Product direction | Ready | Design approved |
| Domain model | Needs migration | Phase 0 baselines first |
| Repository trust | Designed | Snapshot contract tests |
| Policy composition | Designed | Legacy plan equivalence |
| Agent loop | Designed | Typed trajectory tests |
| Tools | Existing, fragmented | Registry migration after engine |
| Evals | Planned | 24 deterministic scenarios |
| GitHub/GitLab | Existing adapters | Snapshot and parity E2E |
| Channels | Frozen | Resume after platform gates |
| Docker/CI | Green baseline | Preserve through every phase |

No unresolved architectural decision blocks Phase 0. Organization packs,
executable validators, durable multi-instance queues, autonomous final approval,
and new channels remain explicitly deferred.

### Runs / Status / Findings

| Review | Runs | Status | Findings |
| --- | ---: | --- | --- |
| Engineering plan review | 1 | clean | 7 structural issues folded into phased design; 0 critical gaps open |
| Architecture | 1 | approved | Preserve primitives; add canonical state, snapshots, plans, and bounded loop |
| Code quality | 1 | approved | Remove duplicated authority before extracting stages |
| Tests | 1 | approved | Deterministic, trajectory, quality, parity, and E2E gates separated |
| Performance/security | 1 | approved | Hard budgets, base trust, confinement, and HIL remain enforced |

VERDICT: CLEARED FOR PHASE 0 IMPLEMENTATION

NO UNRESOLVED DECISIONS
