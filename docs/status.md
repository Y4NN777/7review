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

## Planned Hardening

- Add finding strength classification: confirmed issue, likely issue, note, and
  question.
- Publish inline comments only for confirmed or high-confidence findings.
- Require stronger citation checks for contract/design-backed claims.
- Keep speculative findings in draft summary or human-check sections.
- Build a benchmark set of known reviews with expected true positives, false
  positives, and missed findings.
