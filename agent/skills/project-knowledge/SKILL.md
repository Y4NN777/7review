---
name: project-knowledge
description: Use to discover, classify, select, and cite rich project knowledge for reviews: AGENTS.md, instructions, rules, PRD/SRS, architecture contracts, ADRs, API specs, threat models, design tokens, process docs, and project wiki context packs. Ensures review context is relevant, source-cited, and not invented.
license: Apache-2.0
compatibility: Repository documentation, Markdown, JSON/YAML specs, source tree analysis, optional wiki context packs
allowed-tools: filesystem corpus-selector diff-analyzer headroom mempalace validator
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: project-knowledge
  risk-tier: high
---

# Project Knowledge Skill

## Activation Contract

Activate when a review needs repository guidance, rules, design contracts,
requirements, architecture context, API specs, threat models, process docs,
generated wiki context, or memory-backed project conventions. This skill should
run before model review so context selection is deterministic and citable.

Do not activate for changes that are purely mechanical and have no dependency on
project rules or design inputs, unless the user explicitly asks for knowledge
discovery or documentation.

## Workflow

1. Discover repository guidance files before reviewing changed code.
2. Classify documents by review purpose.
3. Select only knowledge relevant to changed paths, labels, PR/MR text, and touched components.
4. Preserve source file paths and section anchors for citations in findings.
5. Send selected context to compression only after deterministic selection.
6. Record omitted but relevant knowledge as a warning when selection confidence is low.

## Knowledge Categories

- `rules`: `AGENTS.md`, `CLAUDE.md`, `RULES/**`, `.claude/rules/**`
- `planning`: problem statements, PRD, SRS, backlog, sprint planning
- `contract`: invariants, guarantees, prohibitions, legal rules, operation contracts
- `architecture`: responsibility maps, component maps, C4/UML/system model, deployment model
- `api`: OpenAPI, AsyncAPI, REST/WS contract documents
- `security`: threat model, security invariants, sensitive-zone rules
- `design`: design tokens, UI rules, design models, visual contracts
- `delivery`: team process, quality gates, DoD, release constraints
- `wiki`: generated `docs/wiki/**` and `_context/context_pack.json`

## Discovery Rules

Start with conventional project guidance files, then discover rich design inputs by filename and directory intent:

- Agent guidance: `AGENTS.md`, `CLAUDE.md`, `.claude/**`, `.codex/**`
- Team rules: `RULES/**`, `rules/**`, `docs/rules/**`
- Planning docs: paths, names, or headings containing `problem`, `intent`,
  `prd`, `product requirements`, `srs`, `software requirements`,
  `requirements`, `use cases`, `success criteria`, `out of scope`, `backlog`,
  or `sprint`
- Contract docs: paths, names, or headings containing `contract`, `invariant`,
  `constraint`, `law`, `prohibition`, `guarantee`, `must`, `must not`,
  `business rule`, or `error case`
- Responsibility docs: paths, names, or headings containing `responsibility`,
  `ownership`, `where this rule lives`, `component owner`, `boundary`, or
  `module map`
- Architecture/design docs: paths or names containing `architecture`, `design`,
  `system-model`, `data-model`, `deployment`, `adr`, `uml`, `c4`, `sequence`,
  `state`, or `component`
- API contracts: `openapi.yaml`, `openapi.yml`, `openapi.json`, `asyncapi.yaml`, `asyncapi.yml`
- Security docs: paths or names containing `security`, `threat`, `stride`, `risk`, `privacy`
- Design-system assets: paths or names containing `tokens`, `design-system`, `style-guide`, `ui-contract`

Do not hard-code one repository layout. Prefer a project manifest when present; otherwise infer from names, headings, and changed paths.

## Selection Rules

Use changed paths first:

- Language or framework directories -> matching code rules and quality gates
- API/server paths -> API contracts, SRS, architecture, security rules
- Database/migration paths -> data model, invariants, legal/security rules
- UI/client paths -> design tokens, UI contracts, accessibility and client security rules
- Infrastructure/deploy paths -> deployment model, threat model, operational rules
- Auth, identity, crypto, privacy, moderation, storage, or sync paths -> contract and security sources

If no direct path rule exists, use PR/MR title, description, labels, and commit messages to select documents.

## Selection Algorithm

Score each candidate source using these signals:

- changed path overlap with document path or component name
- identifiers shared between PR/MR text and document headings
- SWE Basics chain relevance: problem/PRD -> SRS -> contract/invariant ->
  responsibility -> architecture/model -> implementation/test
- labels such as `security`, `api`, `migration`, `frontend`, `infra`
- file type and domain: API specs for handlers, threat models for auth/privacy,
  runbooks for jobs/deployments, design tokens for UI
- recency and authority: local instructions/rules outrank stale generated docs

Select the smallest set that explains the change. Prefer 3-8 focused sections
over a large undifferentiated document dump.

## Conflict Handling

When sources disagree:

1. Prefer explicit repository instructions over inferred patterns.
2. Prefer current source code over stale docs for observed behavior.
3. Prefer product/security contracts over convenience patterns.
4. Prefer explicit SRS `MUST`/`MUST NOT` rules over broad PRD prose for
   implementation-level findings.
5. Mark unresolved conflicts as review warnings instead of hiding them.

## Citation Rules

Every knowledge-backed finding should cite at least one source path or section.
Use exact paths, document headings, requirement IDs, or context pack section IDs.
Do not cite memory or generated wiki text as authoritative unless it references
the original source.

## Execution Rules

Project-knowledge findings are about missing or misapplied context. File when
the selected corpus clearly governs the changed behavior and the implementation
violates it, or when the review would be unsafe because an expected authoritative
source cannot be found after generic discovery. Suppress if the only support is a
model assumption, uncited memory, stale generated wiki text, or broad repository
prose that does not constrain the changed path.

Choose the knowledge action:

- cite selected source when it directly governs the finding
- request corpus update when docs are missing or stale
- resolve source conflict when two authorities disagree
- use durable memory as advisory context only after current files are checked
- use context compression only after citation metadata is preserved

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Discovery | `filesystem`, `corpus-selector` | Discover rich inputs by names, headings, metadata, identifiers, and changed-path proximity rather than project-specific hard-coding. |
| Selection | `corpus-selector`, `diff-analyzer` | Select sections using changed paths, PR/MR text, labels, component names, and identifiers such as FR, INV, PRO, ADR, DSO, CTRL. |
| Compression | `headroom` | Reduce selected context only after preserving citations, authority order, and required evidence. |
| Memory | `mempalace` | Recall durable conventions and decisions with citations; do not treat memory as stronger than current repository contracts. |
| Validation | `validator` | Reject knowledge-backed findings that lack source path, section, or identifier. |

## Escalation Signals

- The review uses model assumptions where a repository source, wiki context pack, or memory record should be selected.
- Corpus selection is project-specific or path-hard-coded instead of generic classification.
- Conflicting sources are merged silently without authority resolution.

## Evidence Standard

Project-knowledge findings must cite both the changed artifact and the selected
knowledge source that governs it. If the issue is missing knowledge rather than
a violated rule, cite the search signals used and explain why the missing source
reduces review confidence. Never use uncited memory or model inference as the
sole authority for a finding.

## Runtime Integration Checks

- Discovery must start from the configured corpus root or target repository, not the agent source tree unless that is the reviewed repository.
- Selection must combine changed paths, title/body/labels, component names, identifiers, and nearby docs while preserving authority order.
- Context compression may reduce text but must preserve source path, heading, identifier, and why the section was selected.
- Durable memory recall must be scoped to repository/project and treated as lower authority than current source files.

## Review Output Contract

Return a compact selected-context map: source, kind, authority, matching signal, cited identifier, and relevance to changed paths. Findings based on knowledge must include the source citation; missing-knowledge notes must list searched patterns and why the absence matters.

## Technical Patterns To Capture

- ownership boundaries between packages/modules/services
- runtime topology: servers, queues, sidecars, jobs, schedulers
- external dependencies: SCM, databases, model providers, memory, compression
- contract families: requirements, invariants, laws, prohibitions, guarantees
- SWE Basics sources: problem, PRD, SRS, system contract, responsibility map,
  UML/C4 or equivalent architecture model
- operational gates: deploy, rollback, approval, HIL, publishing
- test expectations for critical flows

## False Positive Checks

Do not report a knowledge gap if:

- the change is purely mechanical and does not alter behavior
- the relevant source has no authority over the changed component
- the document is clearly obsolete and contradicted by newer source/docs
- the PR/MR already includes an explicit contract update

## Finding Template

```text
Title: <missing or violated project knowledge>
Source: <path#heading or context-pack section>
Changed path: <file>
Mismatch: <how implementation diverges from source>
Impact: <review correctness, behavior, process, or safety risk>
Fix: <code change or source-of-truth update>
```
