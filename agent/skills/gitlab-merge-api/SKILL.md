---
name: gitlab-merge-api
description: Master GitLab merge request APIs for review enrichment and publishing. Use when the agent needs to normalize GitLab webhook events, fetch MR metadata, commits, diffs, pipelines, discussions, labels, approvals, or publish draft/final merge request notes.
license: Apache-2.0
compatibility: "GitLab API v4, self-managed or GitLab.com merge request workflows"
allowed-tools: scm-api filesystem diff-analyzer validator publisher
metadata:
  version: "2.0.0"
  owner: "7review"
  review-domain: "gitlab-scm"
  risk-tier: "high"
---

# GitLab Merge API Skill

## Activation Contract

Use this skill when the active review provider is GitLab or when code changes touch GitLab webhook parsing, GitLab API clients, MR enrichment, diff retrieval, approval/pipeline context, discussions/notes publishing, idempotent bot notes, token scopes, or self-managed GitLab compatibility.

This skill governs the adapter contract. The model may recommend API-safe behavior, but executable code owns calls, retries, publishing, and validation.

## Review Algorithm

1. Normalize GitLab webhook payloads into `review.Request`.
2. Validate webhook token/secret before trusting object attributes or project identifiers.
3. Enrich from GitLab Merge Requests API before review; do not rely on webhook payload completeness.
4. Fetch commits, diffs/changes, discussions, notes, approvals, and pipelines with pagination.
5. Normalize GitLab-specific fields into provider-neutral review state.
6. Preserve project ID, MR IID, diff refs, discussion IDs, note IDs, and web URLs for traceability.
7. Publish draft/final notes only through the publisher after validation and HIL policy.
8. Re-check prior bot notes immediately before publishing to prevent duplicates in concurrent pipelines.

## Required API Surface

- MR metadata: `GET /projects/:id/merge_requests/:merge_request_iid`
- MR commits: `GET /projects/:id/merge_requests/:merge_request_iid/commits`
- MR changes/diffs: use GitLab MR diff endpoints available for the configured GitLab version.
- MR pipelines: `GET /projects/:id/merge_requests/:merge_request_iid/pipelines`
- MR discussions: `GET /projects/:id/merge_requests/:merge_request_iid/discussions`
- MR notes: `GET /projects/:id/merge_requests/:merge_request_iid/notes`
- Publish note: `POST /projects/:id/merge_requests/:merge_request_iid/notes`
- Publish threaded discussion when line-specific comments are required.

## Technical Patterns

### Webhook Trust Boundary

- Validate the configured GitLab webhook token/secret before decoding trusted business fields.
- Treat `object_kind`, `event_type`, and `object_attributes.action` as routing inputs. Unsupported actions should be ignored or recorded, not partially reviewed.
- Prefer numeric project IDs for API calls because path-encoded project names are easier to mishandle.
- Capture delivery/event metadata when available for audit and deduplication.

### Version and Endpoint Compatibility

- GitLab self-managed installations vary by version. Adapter code should isolate version-sensitive diff endpoints behind one interface.
- If an endpoint is unavailable, degrade explicitly and record missing context; do not silently review an empty diff.
- Page all list endpoints. Large MRs commonly exceed default limits for diffs, commits, notes, and discussions.
- Respect GitLab rate-limit headers and retry only safe reads automatically.

### Diff and Discussion Positioning

- Preserve `old_path`, `new_path`, `new_file`, `renamed_file`, `deleted_file`, and diff refs.
- Inline discussions need valid base/start/head SHAs and a line position accepted by GitLab. If the line cannot be addressed after a rebase or truncated diff, publish a summary note with a file citation.
- Deterministic diff code owns line placement. Model output must not invent GitLab positions.
- Handle binary, generated, and too-large diffs as explicit changed-file states.

### Authentication and Permissions

- Use least-privilege project/group access tokens or deploy tokens appropriate to the deployment model.
- Required scopes commonly include API access for MR reads and notes; production deployments should document the exact token permissions.
- Never log private tokens, webhook secrets, authorization headers, or raw private diffs.

## Normalization Rules

Map GitLab MRs to internal review vocabulary:

- `project_id` -> repository/project ID
- `iid` -> change ID
- `source_branch`, `target_branch` -> branches
- `diff_refs.head_sha` -> source SHA
- `diff_refs.base_sha` / `start_sha` -> target/base SHA
- `author.username` -> author
- labels -> review labels
- changes/diffs -> normalized changed files with old path, new path, patch, status

Preserve GitLab note IDs, discussion IDs, and web URLs for idempotent updates.

## Adapter Contract

GitLab logic belongs in deterministic code, not model instructions. The skill should cause the agent to verify that these responsibilities are implemented by the GitLab adapter:

- Webhook handler: validates the configured token before trusting fields, filters merge request events/actions, tolerates common label payload shapes, captures delivery metadata, and emits `review.Request`.
- Enricher: reads MR metadata, commits, diffs, discussions, notes, pipelines, labels, and approvals through paginated API calls.
- Diff normalizer: preserves old/new paths, rename/delete/new-file state, diff refs, truncation, binary/generated states, and line-addressability.
- Publisher: locates existing review-bot draft/final notes, updates or supersedes safely, and stores note URLs/IDs in run state.
- Compatibility layer: isolates GitLab.com and self-managed endpoint differences behind one tested adapter boundary.

## Failure Modes To Prioritize

- GitLab sends labels as objects while the parser only accepts strings, causing valid webhooks to fail.
- Large MRs lose diffs, notes, or commits because `X-Next-Page` is ignored.
- Self-managed GitLab returns a version-specific diff shape and the agent silently reviews an empty change set.
- Publishing retries create duplicate notes because existing bot markers are not searched immediately before posting.
- Approval or pipeline context is treated as complete when an API failure or permission error actually prevented enrichment.

## Test Expectations

- Token rejection, malformed payload, unsupported event/action, and accepted MR action tests.
- Label normalization tests for arrays of strings and arrays of objects with `title` or `name`.
- Multi-page diffs, commits, notes, discussions, approvals, and pipelines tests using GitLab pagination headers.
- Self-managed compatibility tests for unavailable optional endpoints.
- Draft/final publish idempotency tests using prior bot note markers.

## Execution Rules

GitLab findings should identify which adapter obligation failed: token trust,
MR normalization, paginated enrichment, self-managed compatibility, diff
normalization, approvals/pipeline context, or note idempotency. File when the
adapter can enqueue untrusted work, silently review an incomplete MR, lose
provider IDs needed for publishing, or treat unavailable optional endpoints as
complete context. Suppress generic review-quality comments unless the GitLab
adapter data itself is missing or misleading.

Choose the fix location:

- webhook parser for token, event kind, action, label shape, or project identity
- API client for pagination headers, auth/status handling, endpoint variance, or timeout
- diff normalizer for old/new paths, diff refs, rename/delete/binary/truncation
- publisher for marker lookup, update semantics, final/draft separation, or note URLs
- run state for approval, pipeline, discussion, and partial-enrichment evidence

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Webhook normalization | `scm-api` | Validate token, parse MR event fields, delivery ID, project ID/path, labels, branches, action, and changed path hints. |
| Enrichment | `scm-api`, `diff-analyzer` | Fetch MR metadata, commits, diffs, pipelines, discussions, approvals, and existing bot notes with pagination. |
| Publishing | `publisher` | Use draft/final note markers, update prior notes idempotently, and preserve note IDs or URLs. |
| Validation | `validator` | Reject adapter findings that lack request, endpoint, pagination, auth, or idempotency evidence. |

## Escalation Signals

- Webhook secret validation, event action filtering, delivery dedupe, pagination, or note marker updates are skipped.
- GitLab MR diffs/discussions/pipelines are assumed from webhook payload alone.
- Approval or pipeline status is ignored when final publishing or memory writes depend on it.

## Evidence Standard

Provider findings should cite the exact GitLab boundary: webhook validation, project/MR identity, pagination, diff refs, endpoint compatibility, publishing marker, or token scope. A useful finding identifies how the production agent could miss context, publish the wrong note, duplicate reports, or trust forged input.

## Runtime Integration Checks

- Webhook handling must validate the configured token before accepting object attributes, labels, project identity, branch refs, MR IID, and delivery metadata.
- Enrichment must follow GitLab pagination for diffs, commits, notes/discussions, approvals, and pipelines while tolerating self-managed instances that lack optional endpoints.
- Publishing must update or supersede existing review-bot draft/final note markers and preserve note IDs or URLs in durable run state.
- GitLab-specific IDs such as project ID, path, MR IID, and diff refs must normalize cleanly into provider-neutral request and SCM context.

## Review Output Contract

Report GitLab integration issues with the webhook field, REST endpoint, pagination header, marker, or token scope involved. Include whether the failure causes forged input, missed diffs, stale approvals, duplicate notes, or wrong finalization, plus the local HTTP-server test to add.

## False Positive Checks

- Do not require line discussions for findings that are cross-file, generated, binary, or outside available diff hunks.
- Do not assume GitLab.com-only behavior for self-managed deployments unless the deployment config explicitly pins GitLab.com.
- Do not block on approvals/pipelines being absent if the API legitimately returns no data and the report records that state.
- Do not recommend storing raw API payloads when normalized state and trace IDs provide enough evidence.

## Review Safety

Never assume webhook payloads contain enough context. Always enrich before review. Do not publish duplicate bot notes; find previous bot-authored notes and update, supersede, or append according to publisher policy.

## Finding Template

```md
### [severity] GitLab integration issue

- API boundary: `<webhook/enrichment/diff/publishing/auth>`
- Endpoint or field: `<specific GitLab API/field>`
- Problem: `<failure mode>`
- Evidence: `<changed lines or missing handling>`
- Production impact: `<missed MR data, unsafe note, duplicate report, stale context, auth risk>`
- Suggested fix: `<adapter behavior and test>`
```

Official docs:

- https://docs.gitlab.com/api/merge_requests/
- https://docs.gitlab.com/api/discussions/
