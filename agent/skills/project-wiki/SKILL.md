---
name: project-wiki
description: Use when the agent needs to build, refresh, validate, or consume a repository wiki/context pack for better reviews. This skill turns source trees, docs, contracts, architecture notes, APIs, and tests into evidence-based project knowledge with citations, diagrams, ownership maps, and machine-readable context packs. It is for scheduled/manual knowledge building, not every hot review path.
license: Apache-2.0
compatibility: Repository filesystem access, Markdown docs, optional Mermaid CLI for diagram validation
allowed-tools: filesystem corpus-selector diff-analyzer headroom mempalace validator
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: project-knowledge
  risk-tier: medium
---

# Project Wiki Skill

## Activation Contract

Activate this skill when the user asks to document a repository, build a
DeepWiki-style knowledge base, refresh project knowledge before review, produce a
context pack, validate diagrams, or improve corpus selection with structured
project documentation.

Do not activate this skill for normal per-PR review unless the requested action
is explicitly about project knowledge generation or the existing corpus is
missing/obsolete.

## Purpose

Build durable project knowledge that later review runs can select, compress, and
cite. The output should help a review agent understand architecture, ownership,
contracts, invariants, APIs, data flows, risks, and operational procedures.

## Inputs

Prefer these evidence sources, in order:

1. Existing agent instructions: `AGENTS.md`, `CLAUDE.md`, `agent/instructions.md`
2. Source tree structure and package/module boundaries
3. Public API contracts: OpenAPI, AsyncAPI, protobuf, GraphQL, SDK docs
4. Architecture docs: ADRs, C4 diagrams, deployment docs, component maps
5. Product/process docs: PRD, SRS, runbooks, DoD, release notes
6. Security docs: threat models, controls, privacy and tenant-boundary docs
7. Tests and fixtures that encode behavior not documented elsewhere
8. CI/Docker/config files that define runtime topology

## Output Artifacts

Write or update a wiki directory such as `docs/wiki/` with:

- `README.md`: table of contents and project overview
- `architecture.md`: components, boundaries, data/control flow
- `contracts.md`: APIs, invariants, guarantees, prohibitions
- `operations.md`: deployment, queues, jobs, sidecars, readiness, rollback
- `security.md`: trust boundaries, controls, sensitive assets
- `review-map.md`: which docs/skills matter for common changed paths
- `_context/context_pack.json`: machine-readable sections for review selection

When repository policy requires a different docs location, preserve that local
convention.

## Review Algorithm

1. **Scan structure.** Identify languages, package roots, app entrypoints,
   service boundaries, configs, docs, tests, and generated artifacts.
2. **Build an evidence index.** For every claim, record source path and, when
   possible, heading/function/type names. Avoid uncited architecture claims.
3. **Cluster by responsibility.** Group files into components, adapters,
   domain modules, infrastructure, tests, and docs.
4. **Extract contracts.** Pull out invariants, API semantics, schemas, env vars,
   sidecar dependencies, queues, background jobs, and HIL/approval rules.
5. **Map review relevance.** For each component/path pattern, list the skills
   and wiki sections that should be selected during a code review.
6. **Validate output.** Check Markdown links, Mermaid fences, required sections,
   duplicate headings, and context pack JSON shape.
7. **Summarize changes.** Report created/updated files, stale docs found, and
   unknowns that still need human confirmation.

## Technical Patterns To Capture

### Architecture

- process entrypoints and long-running services
- HTTP routes, webhook handlers, queues, workers, schedulers
- external systems: SCM, databases, object stores, sidecars, model providers
- package ownership boundaries and forbidden cross-dependencies
- generated code and files that should not be hand-edited

### Contracts

- public request/response/event schemas
- persistent data shapes and migrations
- environment variables and config defaults
- HIL approval boundaries
- idempotency markers for publishing or external writes
- model/tool interface schemas

### Review Selection Map

Produce path-to-context guidance such as:

```text
agent/app/** -> API contract, security, reliability, review-publisher
agent/tools/github.go -> github-merge-api, api-contract, reliability
docker/** -> config-dependency, reliability, security
docs/openapi.yaml -> api-contract, traceability
migrations/** -> data-migration, reliability, security
```

Do not hard-code these examples; infer the actual project map.

## Execution Rules

Project-wiki findings are about generated knowledge quality. File when a context
pack, wiki section, or review map would mislead future reviews because it lacks
citations, invents architecture, misses a key lifecycle surface, or cannot be
rediscovered by generic corpus selection. Suppress when the repository already
has a different authoritative documentation shape and review selection can still
find it.

Choose the correction:

- add citation/source metadata when claims are real but unsupported
- update the context-pack field when lifecycle, API, guard, or ownership facts changed
- mark uncertainty when code/docs conflict and no authority resolves it
- adjust generic discovery signals when generated content is valid but unreachable
- avoid memory persistence until generated knowledge is approved and source-backed

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Repository scan | `filesystem`, `corpus-selector` | Discover docs, contracts, ADRs, APIs, package boundaries, tests, commands, config, and ownership signals. |
| Context extraction | `diff-analyzer`, `filesystem` | Capture stable architecture, workflows, invariants, interfaces, and traceability identifiers with citations. |
| Compression/indexing | `headroom`, `mempalace` | Compress large context packs and persist durable project decisions only when validated and cited. |
| Validation | `validator` | Verify generated context packs against schema, source citations, and stale-source rules. |

## Escalation Signals

- Generated wiki content lacks citations, source paths, update time, or authority level.
- The context pack invents architecture not present in code/docs.
- Memory or embeddings preserve unapproved, stale, or private review content.

## Evidence Standard

- Every non-obvious statement must cite a source path.
- Do not invent components that are not visible in source/docs.
- Mark uncertain conclusions as `Needs confirmation`.
- Prefer nearby source code over stale documentation when they conflict.
- Keep generated wiki text concise enough to be useful as review context.

## Runtime Integration Checks

- Wiki generation must read from the configured repository/corpus root and produce artifacts that the corpus selector can rediscover without hard-coded project names.
- Context packs must preserve source paths, headings, identifiers, review triggers, and freshness metadata so context compression can reduce them without destroying citations.
- Durable memory should store project decisions from approved outputs only; generated wiki summaries remain lower authority than cited source files.
- Scheduled or manual wiki refresh must not block hot webhook reviews unless the current review explicitly depends on a missing or stale context pack.

## Review Output Contract

When reporting wiki issues, identify the missing or stale artifact, the source file or citation gap, the review workflow affected, and the validation needed. Do not ask for broad documentation rewrites; request the smallest page, context-pack field, diagram, or review trigger needed for accurate future reviews.

## False Positive Checks

- Do not flag a missing wiki page as blocking if the repository already has a
  different authoritative documentation structure.
- Do not rewrite correct local terminology into generic architecture labels.
- Do not treat generated wiki output as more authoritative than cited source
  files.
- Do not require diagrams for simple repositories where text and source
  citations explain the architecture clearly.

## Context Pack Shape

`_context/context_pack.json` should use this shape:

```json
{
  "project": "name",
  "generated_at": "RFC3339 timestamp",
  "sections": [
    {
      "id": "architecture.webhooks",
      "title": "Webhook flow",
      "kind": "architecture",
      "paths": ["agent/app/gitlab.go"],
      "content": "Short evidence-based summary",
      "review_triggers": ["webhook", "signature", "queue"]
    }
  ]
}
```

## Validation Checklist

- `README.md` links to all wiki pages.
- Each page has cited source paths.
- Mermaid diagrams parse when Mermaid tooling is available.
- Context pack is valid JSON.
- Review map covers major changed-path families.
- Unknowns are listed instead of hidden.
- No secrets or private tokens are copied into docs.

## Finding Template

When this skill is used during review, report wiki/documentation issues only if
they affect review correctness, onboarding, operational safety, or traceability.
Do not block normal code changes just because a wiki page could be prettier.

```text
Title: <missing or stale project knowledge>
Artifact: <wiki page/context pack/source doc>
Evidence: <source path, missing citation, invalid context pack field, or stale claim>
Impact: <review selection, onboarding, operations, traceability, or safety risk>
Fix: <specific wiki/context-pack/source update>
Validation: <link check, JSON schema check, diagram parse, or citation check>
```
