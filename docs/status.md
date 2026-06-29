# Current Status

This document tracks the current implementation state and operational limits of
7review. It intentionally avoids project names, merge request IDs, tokens,
private URLs, and other deployment-specific details.

## Implementation State

7review currently supports:

- GitHub pull request and GitLab merge request webhook intake.
- Bounded in-process webhook workers for concurrent reviews.
- SCM enrichment for metadata, changed files, diffs, discussions, and publish
  positions.
- Repository knowledge selection through an in-process document graph.
- Portable `SKILL.md` review procedures with required core/provider coverage.
- Model routing through OpenAI, Anthropic, OpenRouter, DeepSeek, Mistral,
  Gemini, Ollama, and OpenAI-compatible endpoints.
- Provider-native read-only tool calls inside the review loop.
- Deterministic validation for required finding fields, confidence, changed-file
  location, and addressable inline comment positions.
- Draft report publishing, inline draft comments, human approval, final
  publishing, and approved memory writeback.
- Operator CLI/TUI/chat workflows for setup, status, run inspection, reruns,
  approval, final publishing, and memory review.

## Latest Smoke Coverage

A live GitLab merge request smoke run completed end-to-end with:

- webhook acceptance and queue processing
- SCM enrichment
- graph-based repository context selection
- skill selection and required skill coverage
- model tool calls for changed files, diff summary, and merge request metadata
- accepted model findings
- inline draft comments published to changed lines
- draft report publication

The smoke run proves the runtime path and publishing path are functional. It
does not prove that every model finding is correct.

## Review Quality

The strongest observed results are on changes where the repository contains
stable documentation anchors: requirement IDs, contract rules, API routes,
schemas, data-model sections, design decisions, or ownership docs.

The current system is good at surfacing:

- contract/API/data-model drift
- missing traceability between code and repository docs
- changed-line findings that can be published inline
- run audit data through timelines, selected context manifests, tool
  observations, provider traces, and draft reports

The current system is weaker at distinguishing:

- confirmed defects from likely defects
- review notes from publishable findings
- speculative performance or future-maintenance concerns from concrete issues

## Known Limits

- Model quality varies significantly by provider and model.
- A structurally valid model finding can still be too speculative.
- Inline comments should stay limited to addressable, high-confidence findings.
- Positive observations and weak concerns should be summary notes, not inline
  findings.
- Final publication should remain human-approved.

## Review Quality Roadmap

The next quality pass should treat 7review as an assisted draft reviewer, not as
an autonomous final reviewer. The goal is to reduce false positives, preserve
useful review notes, and make source-of-truth authority explicit.

### Source-Of-Truth Authority

The document graph should expose authority as a first-class signal, not only as
section kind or selection score.

Recommended authority levels:

- `sot`: binding source of truth, such as requirements, contracts, API specs,
  data models, security rules, and approved repository rules.
- `decision`: ADRs and approved architecture decisions.
- `implementation_context`: ownership docs, runbooks, operational notes, and
  code-adjacent documentation.
- `design_context`: design docs, tokens, accessibility rules, states, and
  component behavior.
- `supporting`: useful references that cannot justify a finding alone.
- `memory`: approved historical memory, always lower authority than repository
  files.

The `evidence_manifest` should explain:

- why the section was selected
- which review signal pulled it in
- which authority level it has
- whether it can justify a finding by itself
- whether it only supports another source

### Finding Strength

The validator should classify every model issue before publishing:

- `confirmed`: direct evidence in changed code plus a cited source-of-truth
  rule.
- `likely`: strong evidence, but part of the needed context is absent or
  inferred.
- `speculative`: hypothesis, future debt, unmeasured performance concern, or
  risk without a concrete violated rule.
- `note`: useful non-blocking observation or positive context.
- `question`: a clarification needed from the author.

Only `confirmed` findings, and a narrow subset of high-confidence `likely`
findings, should become inline comments by default.

### Skill-Specific Strictness

Some skills need stricter reporting rules:

- Data migrations should not report TTL, pruning, or performance risks unless
  there is volume evidence, a known slow query, a missing required index, or an
  explicit repository requirement.
- Contract drift is strong when a changed field, enum, route, event, or schema
  is ratified in code but missing from the API or schema source of truth.
- Design decisions should not be inverted into defects. If an ADR or system
  model allows a nullable relation or temporary gap, the review should only
  report a missing follow-up when another source requires that follow-up.
- Ownership and runbook docs should guide maintainability notes, but should not
  create blocking findings without a violated source-of-truth rule.

### Findings, Notes, And Questions

The report model should separate:

- `findings`: actionable bugs or violated requirements.
- `notes`: useful observations, positive confirmations, or low-risk
  maintainability context.
- `questions`: points that need author clarification.

This prevents weak concerns from being published as inline defects while still
keeping useful reviewer context in the draft.

### Citation Validation

A strong finding should include:

- changed file and changed line
- exact source document or section
- violated rule restated in the finding
- explanation of how the diff violates that rule

If any of these are missing, the system should downgrade the issue to `likely`,
`note`, or `question`, or reject it when it is not useful.

### Publish Policy

Default publication should be:

- draft summary for all accepted findings, notes, and questions
- inline comments only for `confirmed` findings on addressable changed lines
- `likely` findings inline only when confidence and citations are strong
- speculative items kept in a "Needs human check" section
- final publication always behind human approval

### Benchmark Reviews

Create a small benchmark set of known reviews and expected outcomes. Each case
should measure:

- true positives
- false positives
- missed findings
- citation quality
- correct downgrade of speculative concerns
- correct use of source-of-truth authority
