---
name: review-publisher
description: Publish draft and final review reports safely to GitHub or GitLab. Use when the agent needs to format findings, avoid duplicate bot comments, create inline comments, update previous reports, or apply human-in-the-loop approval policy before publishing.
license: Apache-2.0
compatibility: "GitHub issue/PR comments, GitHub review comments, GitLab MR notes/discussions, approval-gated publishing"
allowed-tools: publisher scm-api run-store validator mempalace
metadata:
  version: "2.0.0"
  owner: "7review"
  review-domain: "publishing"
  risk-tier: "high"
---

# Review Publisher Skill

## Activation Contract

Use this skill when code changes touch report rendering, SCM comment publishing, inline comment placement, idempotency markers, draft/final lifecycle, HIL decisions, redaction, source citations, model metadata, or user-visible review formatting.

Publishing is a side-effect boundary. It must be deterministic, idempotent, and policy-gated. The LLM may draft content but must not decide raw API mutations.

## Review Algorithm

1. Render validated findings into deterministic Markdown.
2. Search previous bot comments before posting.
3. Publish draft reports only when configured for draft/HIL mode.
4. Publish final reports only after approval or when policy allows auto-final.
5. Include traceability citations and model/provider metadata.

## Idempotency

Use a stable marker in bot comments:

```md
<!-- 7review:project=<project-id>;change=<change-id>;kind=draft -->
```

Before publishing:

- find existing marker for the same project/change/kind
- update if the SCM API supports it
- otherwise add a superseding comment and mark the old one stale

## Technical Patterns

### Report Rendering

- Sort findings deterministically by severity, confidence, file, and stable ID.
- Keep finding IDs stable across rerenders when the underlying issue is the same.
- Separate blocking findings, advisory findings, suppressed/rejected findings, validation notes, and context limitations.
- Include the selected skills/corpus sources used for the review so engineers can understand the review basis.
- Do not expose chain-of-thought, raw prompts, raw provider JSON, secrets, or large private diff blocks.

### Draft and Final Reports

- Draft reports must be visually and machine-marked as draft.
- Final reports must reference approval or policy that allowed final publication.
- Human edits should appear in the final report without pretending they were model-generated.
- If a new run supersedes an old draft, mark the old draft stale or update it in place.

### Provider Differences

- GitHub summary comments usually use issue comments on the PR; inline findings may use pull request review comments when a valid diff position exists.
- GitLab summary comments usually use MR notes; inline findings may use discussions when valid diff refs and positions exist.
- Provider-specific publishing failures should not corrupt normalized report state. Record failure metadata and retry through the publisher.

### Redaction and Privacy

- Redact tokens, authorization headers, webhook secrets, private keys, and credentials before rendering.
- Quote only the minimal code snippet needed to prove a finding.
- Avoid copying full private payloads, model prompts, or memory contents into SCM comments.

### Streaming and Chat Follow-Up

- Streaming chat can explain or refine a draft, but persistent report changes must be captured as structured actions.
- Chat-driven "publish", "approve", "suppress", or "revise" commands must pass through HIL and publisher validation.

## Report Sections

- Summary
- Blocking findings
- Non-blocking findings
- Traceability gaps
- Security/design/API contract risks
- Validation notes
- Sources used
- Provider/model footer

Do not include raw secrets, full private diffs beyond necessary snippets, or unvalidated model output.

## Publishing State Machine

Treat publishing as a stateful side effect with deterministic inputs:

1. `render_draft`: convert validated findings and context limitations into Markdown.
2. `publish_draft`: create/update the provider draft marker when policy allows draft visibility.
3. `await_hil`: store draft and block final publication until approval or policy auto-final.
4. `render_final`: incorporate human decisions, suppressions, and approval metadata.
5. `publish_final`: create/update the final provider marker and store provider URL/ID.
6. `memory_proposal`: hand final approved knowledge to durable memory only after final semantics are satisfied.

Every step must be replay-safe. If a process restarts between steps, durable run state should let the publisher continue without duplicate comments.

## Marker Contract

Use markers that machines can parse and humans can ignore:

- Include provider, project/repository, change ID, run ID, report kind, and schema version when possible.
- Keep draft and final markers distinct.
- Never depend only on report title text for idempotency.
- If a provider cannot update existing comments, create a new comment that explicitly supersedes the prior marker.
- Preserve old marker IDs/URLs in run state for audit.

## Test Expectations

- Renderer snapshot or structural tests for empty findings, blocking findings, advisory findings, suppressed findings, and context limitations.
- Redaction tests for tokens, headers, secrets, private keys, and webhook payload fragments.
- GitHub/GitLab idempotency tests for existing draft/final markers.
- Failure tests proving provider publish errors do not corrupt stored draft/final report content.
- HIL tests proving final publishing cannot run from a draft-only state.

## Execution Rules

Publisher findings are side-effect and report-integrity findings. File when the
rendered report can include unvalidated/rejected content, leak secrets, lose
source citations, publish to the wrong provider object, duplicate comments on
retry, or finalize without HIL state. Suppress pure wording preferences unless
the wording changes machine markers, operator meaning, or required evidence.

Choose the repair location:

- renderer for finding grouping, redaction, citations, and report sections
- marker lookup/update for idempotent draft/final publishing
- run-store transition for durable publish state and provider IDs
- provider adapter for GitHub/GitLab-specific update constraints
- durable-memory handoff for approved final knowledge only

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Report source | `run-store`, `validator` | Read validated draft/final report state and reject unsupported or stale findings before publishing. |
| Existing marker lookup | `publisher`, `scm-api` | Find prior bot draft/final markers and update them idempotently instead of duplicating comments. |
| Finalization | `run-store`, `publisher` | Confirm approval/finalizing state before final publish and store provider IDs/URLs afterwards. |
| Memory handoff | `mempalace` | Propose durable memory only from approved final content with report citations. |

## Escalation Signals

- Draft and final publishing use the same marker or overwrite each other.
- Publishing does not check durable run state or validated report content.
- Retries can create duplicate bot comments, lose provider IDs, or publish rejected findings.

## Evidence Standard

Publishing findings should cite the exact rendering or side-effect boundary: marker format, duplicate-detection query, provider update endpoint, redaction path, HIL state check, or inline-position validation. A strong finding explains what duplicate, leak, stale report, or unauthorized final publish would occur.

## Runtime Integration Checks

- Renderer output must be built from validated findings and run metadata, not raw model text or untrusted PR/MR comments.
- Draft and final publishing must use separate markers and provider-specific update paths while sharing provider-neutral report content.
- Final publishing must read approved durable state and preserve provider IDs/URLs for audit and retry behavior.
- Memory proposals should use final approved reports only and must not include suppressed, rejected, or redacted material.

## Review Output Contract

Publishing findings must identify the exact report stage, marker, provider action, and durable-state field at risk. Include whether the bug causes duplicate comments, stale final reports, leaked secrets, invalid inline positions, or skipped HIL.

## False Positive Checks

- Do not require inline comments for findings that are cross-cutting, generated, binary, outside the diff, or based on corpus rather than a changed line.
- Do not require exact formatting beyond stable sections and markers unless a consumer depends on it.
- Do not block publishing because an optional provider metadata field is absent if the report records that context honestly.
- Do not treat draft publishing as unsafe when policy explicitly allows draft comments and the marker is clear.

## Finding Template

```md
### [severity] Review publishing issue

- Publishing path: `<render/draft/final/inline/idempotency/redaction>`
- Provider: `<github/gitlab/generic>`
- Problem: `<duplicate, leak, skipped HIL, unstable report, bad marker, invalid line>`
- Evidence: `<changed lines or missing guard>`
- Production impact: `<engineer confusion, unsafe final report, secret exposure, noisy SCM>`
- Suggested fix: `<deterministic render/publisher behavior and test>`
```
