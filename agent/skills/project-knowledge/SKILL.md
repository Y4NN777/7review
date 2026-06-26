---
name: project-knowledge
description: Load and select rich project knowledge for reviews, including SDLC planning docs, SRS, architecture contracts, ADRs, API specs, threat models, design tokens, AGENTS.md, CLAUDE.md, and RULES directories. Use when reviewing projects whose correctness depends on design and process documents, not only code diffs.
---

# Project Knowledge Skill

## Workflow

1. Discover repository guidance files before reviewing changed code.
2. Classify documents by review purpose.
3. Select only knowledge relevant to changed paths, labels, PR/MR text, and touched components.
4. Preserve source file paths and section anchors for citations in findings.
5. Send selected context to compression only after deterministic selection.

## Knowledge Categories

- `rules`: `AGENTS.md`, `CLAUDE.md`, `RULES/**`, `.claude/rules/**`
- `planning`: PRD, SRS, backlog, sprint planning
- `contract`: invariants, guarantees, prohibitions, legal rules, operation contracts
- `architecture`: component maps, C4/UML/system model, deployment model
- `api`: OpenAPI, AsyncAPI, REST/WS contract documents
- `security`: threat model, security invariants, sensitive-zone rules
- `design`: design tokens, UI rules, design models, visual contracts
- `delivery`: team process, quality gates, DoD, release constraints

## Discovery Rules

Start with conventional project guidance files, then discover rich design inputs by filename and directory intent:

- Agent guidance: `AGENTS.md`, `CLAUDE.md`, `.claude/**`, `.codex/**`
- Team rules: `RULES/**`, `rules/**`, `docs/rules/**`
- Planning docs: paths or names containing `planning`, `prd`, `srs`, `requirements`, `backlog`, `sprint`
- Contract docs: paths or names containing `contract`, `invariant`, `law`, `prohibition`, `guarantee`
- Architecture/design docs: paths or names containing `architecture`, `design`, `system-model`, `data-model`, `deployment`, `adr`
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
