---
name: methodology-review
description: Use to enforce an engineering method before judging code: problem intent, PRD/product scope, SRS/verifiable requirements, system contract/invariants, responsibility assignment, architecture/modeling, then implementation review. Also enforces the agent review lifecycle when the target system is a review automation service. Prevents shortcut reviews that let code or model output invent missing context.
license: Apache-2.0
compatibility: Any software repository with requirements, product intent, system contracts, architecture docs, or implementation changes
allowed-tools: scm-api filesystem diff-analyzer corpus-selector headroom mempalace llm validator publisher run-store
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: methodology
  risk-tier: high
---

# Methodology Review Skill

## Activation Contract

Activate when reviewing any non-trivial behavior, architecture, data, API,
security, UI, workflow, or deployment change. This skill protects the review
from jumping straight to code without first checking the product intent,
requirements, system contract, invariants, responsibility placement, and
architecture model that should govern the implementation.

When the target repository is itself an automation/review agent, also check its
runtime lifecycle. Otherwise do not assume the target has SCM, HIL, memory, or
publishing surfaces.

## SWE Basics Before Code Method

Use this method as a portable engineering sequence:

1. **Problem / Intent**: identify the problem the system is solving and what is
   out of scope.
2. **PRD**: find product users, use cases, success criteria, and exclusions.
3. **SRS**: find verifiable functional requirements, business rules,
   non-functional constraints, and error cases.
4. **System Contract**: extract guarantees, prohibitions, invariants, and
   technical constraints.
5. **Responsibility Assignment**: decide where each rule belongs: UI, API,
   domain service, data store, background job, external integration, or
   deployment boundary.
6. **Architecture / Modeling**: map structure and behavior using ADRs,
   component maps, UML/C4, sequence/state diagrams, or equivalent local docs.
7. **Implementation Review**: only then judge changed code and tests.

If any earlier layer is missing, file a gap only when the missing layer makes
the code review unsafe, ambiguous, or likely to accept an incorrect design.

## Generic Review Lifecycle

Follow this order for any repository:

1. Locate the changed behavior and affected surfaces.
2. Select relevant PRD/SRS/contract/architecture/process sources.
3. Map requirements to responsibilities and components.
4. Compare implementation, tests, and docs against those sources.
5. Validate findings with concrete code and source evidence.
6. Render findings with remediation that says whether to fix code, tests, docs,
   or the design contract.

For review-automation systems, additionally check event intake, enrichment,
diff/corpus selection, model/tool execution, validation, publishing, approval,
and memory lifecycle.

## Execution Contract

Do not skip the upstream engineering layer to "save time." If product intent,
requirements, contracts, invariants, or responsibility boundaries are present,
use them before judging code. If they are absent, say what was searched and keep
severity proportional to the risk created by the absence.

## Model Allocation

- Discovery and selection: deterministic tooling where available.
- Requirement/contract reasoning: large reasoning model when relationships are
  non-trivial.
- Report formatting: smaller formatter model when supported.
- Memory and context compression: use only when the runtime provides those
  integrations and citations survive.

## Review Discipline

Do not let the model invent context. The model should reason over selected
diffs, selected PRD/SRS/contract/architecture sources, selected skills, and
cited memory when available.

## Stage Checks

### Event and SCM Enrichment

- Webhook secret/signature must be validated before enqueue.
- Event payload is normalized into `review.Request`.
- SCM API enrichment must fetch metadata, commits, changed files, discussions,
  checks/pipelines, labels, and approvals when provider supports them.
- Review must not rely only on webhook payload for files or SHAs.

### Diff and Corpus

- Changed files must be normalized with stable paths and token estimates.
- Large patches should be chunked before model review.
- Generated, vendored, lock, and minified files should be skipped or deweighted
  by policy.
- Corpus selection must use changed paths, title/body/labels, and identifiers
  such as `FR-*`, `ADR-*`, `INV-*`, `PRO-*`.

### Memory and Context Reduction

- Durable memory recall failure is a hard review failure when memory is required by the selected workflow.
- Context compression failure is a hard review failure when compression is required by the selected workflow.
- Reduced context must preserve source identity: path, title, kind, and section.
- Memory write happens only after final/HIL approval.

### Model Review and Validation

- Model output must be structured and parseable.
- Findings must be validated for severity, confidence, duplicate IDs, and changed
  path relevance.
- Draft report must be rendered from validated findings, not raw LLM text.
- Provider/model metadata should be recorded for traceability.

### Publishing and HIL

- Draft publish is allowed after deterministic validation.
- Final publish requires explicit approval when policy requires HIL.
- Human rejected findings must not enter final report or memory.
- Publisher must be idempotent through stable bot markers.

## Anti-Patterns

- "Review the diff directly" without SCM enrichment.
- Sending all repository files to the model instead of selected corpus.
- Treating chat messages or model replies as approval.
- Writing memory from draft or rejected findings.
- Letting optional/no-op adapters hide required production dependencies.
- Publishing raw model JSON or unvalidated findings.

## Execution Rules

Methodology findings are lifecycle break findings. File when a deterministic
stage is skipped, reordered, hidden behind model inference, or disconnected from
durable state in a way that changes review correctness. Suppress when a stage is
implemented under a different concrete package name but still preserves the same
event-to-memory semantics and tests cover the connection.

Choose the broken stage before writing:

- intake/SCM for trust, normalization, and enrichment gaps
- diff/corpus for missing changed-file or knowledge selection
- memory/headroom for lost citations, scope, or required dependency behavior
- model/validator for raw or unsupported findings
- report/HIL/publisher for approval, idempotency, and state transition issues

## Tool Routing

| Lifecycle Step | Tool Surface | Required Use |
| --- | --- | --- |
| Event and SCM enrichment | `scm-api` | Normalize webhook request, validate provider secret/signature, fetch commits, diffs, discussions, checks/pipelines, and metadata. |
| Diff and corpus | `diff-analyzer`, `filesystem`, `corpus-selector` | Normalize changed files, chunk patches, select relevant rules/contracts/docs, and cite sources. |
| Memory and compression | `mempalace`, `headroom` | Recall durable decisions, then reduce context without dropping required evidence. |
| Review and validation | `llm`, `validator` | Model review must consume selected context; deterministic validation filters unsupported findings. |
| Report and lifecycle | `publisher`, `run-store` | Store draft/final state, publish idempotently, enforce HIL, then propose memory only after final semantics. |

## Escalation Signals

- Any lifecycle stage is skipped, mocked, or inferred from model text when a deterministic integration exists.
- The model sees a diff without SCM enrichment, selected project knowledge, or memory recall.
- Publishing, HIL, or memory writes happen outside durable run state.

## Evidence Standard

Methodology findings must name the skipped or weakened lifecycle stage, the
artifact affected, and the incorrect downstream behavior it can cause. Cite the
changed path and the state transition or data flow that proves the shortcut.

## Runtime Integration Checks

- For review-automation systems, confirm the runtime follows one connected path: event, SCM enrichment, diff, corpus/skills, durable memory recall, context compression, model review, validation, report, HIL/publish, and memory proposal.
- GitHub and GitLab should share the lifecycle after normalization while retaining provider-specific enrichment and publishing adapters.
- Required services must appear in config, runtime packaging, readiness, tests, and operator status; no silent no-op in production mode.
- Skills are inputs to model reasoning and review procedure, while code owns deterministic API calls, state transitions, publishing, and validation.

## Review Output Contract

Methodology findings must name the skipped or reordered lifecycle boundary and the artifact that proves it: source state, selected corpus, selected skill, reduced context, validation result, report, publish action, or memory proposal. Include the test that would fail if the lifecycle regresses.

For `confirmed` findings, include structured citations: `source`,
`heading_or_key`, `rule`, and `violation`. The `rule` must match selected
repository evidence, and `violation` must explain how the changed code or state
transition breaks that rule. If the rule cannot be cited this way, use
`finding_type=note` or `strength=likely`.

## False Positive Checks

- Do not report a missing stage if the change is outside the review lifecycle.
- Do not require model use for deterministic stages that code can solve.
- Do not treat a deliberate test fake as a production no-op adapter.
- Do not require final publishing when policy explicitly keeps a run as draft.

## Finding Template

```text
Title: <lifecycle shortcut or skipped gate>
Stage: <event|scm|diff|corpus|memory|headroom|model|validation|publish|hil>
Evidence: <changed file/path and code path>
Risk: <what incorrect review, unsafe publish, or memory corruption can happen>
Fix: <restore deterministic stage or gate>
Test: <unit/integration check for the lifecycle invariant>
```
