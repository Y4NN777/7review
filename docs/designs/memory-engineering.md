# Design: Governed Review Memory

Status: APPROVED FOR IMPLEMENTATION PLANNING
Date: 2026-08-28

## Decision

Keep MemPalace as the first semantic retrieval index, but move all review
semantics into a provider-neutral 7review memory layer. Governed records are
linked to the run-scoped Review Evidence Graph. MemPalace is not the source of
truth and may be replaced or rebuilt without changing review behavior.

Do not copy Hermes' compact `MEMORY.md` model as the primary store. Adopt its
useful separation between always-on curated facts, searchable history, and
procedural skills, plus its provider lifecycle and strict size/security gates.
7review additionally needs typed records, repository scope, evidence lineage,
feedback outcomes, contradiction handling, and quality evaluation.

## Current Gap

The current `MemoryStore` accepts a review request and returns three untyped
string lists: conventions, decisions, and history. Approved writes include
finding titles and the complete final report, with positional vector IDs such as
`convention-1`. This loses source revision, scope, authority, outcome, freshness,
and supersession, while repeated reports create retrieval noise.

HIL approval and the rule that memory cannot justify a blocking finding alone
are correct and remain hard invariants.

## Memory Model

7review distinguishes five memory classes:

1. **Semantic:** accepted conventions, architectural decisions, and stable facts.
2. **Episodic:** prior review hypotheses, observations, findings, and outcomes.
3. **Feedback:** accepted, rejected, revised, duplicate, and no-finding results.
4. **Procedural candidates:** repeated successful review strategies awaiting
   promotion into repository-owned packs, rules, or `SKILL.md` methods.
5. **Operational:** provider failures and execution behavior, isolated from
   defect judgment.

Each `MemoryRecord` carries a stable ID, kind, organization/repository/domain/
module/feature/path scope, content, evidence references, base/head revisions,
authority, confidence, status, timestamps, model/policy provenance, and links to
records it supersedes or contradicts. Status is one of `proposed`, `active`,
`superseded`, `contradicted`, `expired`, or `archived`.

The immutable run ledger remains the audit source for complete trajectories.
Curated memory stores compact derived knowledge and references the ledger; it
does not duplicate full reports or hidden chain-of-thought.

Memory does not create a global software knowledge graph. Its relations connect
governed records to review entities such as repository, domain, module, feature,
rule, finding, HIL decision, and source run.

## Architecture

```text
ReviewPlan + diff + Review Evidence Graph
                    |
                    v
             MemoryEngine (7review)
 capture -> validate -> retrieve -> rank -> propose -> consolidate
       |                    |                    |
       v                    v                    v
 run ledger + records   evidence-graph       approval/promotion
                         recall nodes
                              |
                 +------------+------------+
                 |                         |
           exact scoped index       MemPalace semantic index
           authoritative records    stable record references
```

`agent/memory` owns domain types, policy, ranking, lifecycle, and metrics.
Governed records and links are authoritative; MemPalace stores embeddings and
stable record references as a rebuildable secondary index. `agent/tools`
keeps the MemPalace HTTP adapter and embedding integration.

Recall is compiled from `ReviewPlan`, changed paths, repository identity, and
trusted revision. Exact path/rule/module matches run first; semantic search runs
second. The engine then deduplicates, filters by scope and lifecycle, applies
authority/freshness limits, enforces a token budget, and records why every item
was selected. Repository files and base-revision policy always outrank memory.
Selected records become evidence-graph nodes with `recalled_because`,
`applies_to`, and source-run links. They remain supporting evidence.

## Learning And Governance

After HIL, 7review derives typed proposals from accepted findings, rejected or
revised findings, human notes, and relevant no-finding outcomes. Before write it
redacts secrets, scans untrusted instructions, validates evidence references,
uses idempotent stable IDs, and detects duplicates, contradictions, and
supersession. Activation requires explicit human approval or a narrowly scoped
operator policy; final review approval alone is not blanket memory approval.

Accepted, rejected, revised, and duplicate outcomes are taken from persisted
evidence paths. Rejected findings create feedback relations and suppression
signals, never repository conventions. Contradictions preserve both records and
their evidence until an authorized supersession decision is recorded.

Memory may tune retrieval priority and propose review-method changes, but it may
not silently mutate repository policy, tool permissions, severity, or
publication rules. A procedural candidate becomes durable methodology only
through a generated repository patch reviewed and merged like normal code.

Consolidation creates a compact replacement record, archives its sources, and
retains bidirectional lineage. Decay lowers retrieval priority rather than
deleting audit-relevant evidence. Revalidation against newer repository truth
can supersede or contradict old records.

## Evaluation

Scenario runs compare memory disabled and enabled. Required metrics are finding
precision/recall delta, false-positive recurrence, useful recall rate, harmful
or stale recall rate, citation validity, contradiction rate, memory utilization,
latency, context cost, and promotion accuracy. More recalled text is never a
success metric.

Release gates require deterministic tests for isolation, ranking, lifecycle,
idempotency, redaction, approval, and provider failure. MemPalace outage must
degrade to a review without semantic history, not fail repository acquisition or
weaken mandatory checks.

Graph-specific gates verify that a memory-only path cannot confirm a finding,
recalled records point to existing governed records, and stale or contradicted
records are removed before prompt construction.

## Research Basis

- Hermes separates compact factual memory, searchable session history, and
  on-demand procedural skills, and exposes lifecycle hooks through a pluggable
  memory provider.
- Hermes' own structured-memory proposal identifies the limits of flat files;
  its separate local and external retrieval paths also warn against duplicated
  memory systems.
- MemPalace provides useful backend mechanisms including MMR retrieval,
  supersession, pinning, and consolidation, while leaving knowledge generation
  to the caller.
- Agent-memory research distinguishes semantic, episodic, and procedural memory
  and supports learning reusable procedures from supervised trajectories.

References:

- https://github.com/NousResearch/hermes-agent/blob/main/website/docs/guides/work-with-skills.md
- https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/sessions.md
- https://github.com/NousResearch/hermes-agent/blob/main/agent/memory_provider.py
- https://github.com/NousResearch/hermes-agent/issues/346
- https://github.com/NousResearch/hermes-agent/issues/29901
- https://github.com/MemPalace/mempalace/issues/595
- https://github.com/MemPalace/mempalace/blob/develop/MISSION.md
- https://arxiv.org/abs/2512.13564
- https://arxiv.org/abs/2508.06433

## Non-Negotiable Invariants

- Memory is advisory and cannot independently confirm a finding.
- Every recalled claim is scoped, attributable, explainable, and bounded.
- Untrusted PR/MR text never becomes active memory without validation.
- Secrets and hidden reasoning are never stored.
- Rejected findings are learning signals, not conventions.
- MemPalace results are rehydrated and revalidated against governed records.
- Memory relations enrich the Review Evidence Graph; they do not create a
  parallel graph authority.
- Provider failure cannot weaken repository-owned review requirements.
- Policy and procedural promotion always remain reviewable repository changes.
