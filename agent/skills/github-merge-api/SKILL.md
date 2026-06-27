---
name: github-merge-api
description: Master GitHub pull request APIs for review enrichment and publishing. Use when the agent needs to normalize GitHub webhook events, fetch PR metadata, commits, changed files, checks, discussions, labels, reviewers, or publish draft/final review comments.
license: Apache-2.0
compatibility: "GitHub REST API v3, GitHub webhooks, GitHub App or fine-grained token authentication"
allowed-tools: scm-api filesystem diff-analyzer validator publisher
metadata:
  version: "2.0.0"
  owner: "7review"
  review-domain: "github-scm"
  risk-tier: "high"
---

# GitHub Merge API Skill

## Activation Contract

Use this skill when the active review provider is GitHub or when code changes touch GitHub webhook parsing, GitHub API clients, PR enrichment, check/status collection, review comment publishing, inline comment placement, idempotency markers, token scopes, or retry/rate-limit behavior.

This skill is API-operational: it guides how the agent gathers evidence and publishes review output. It must not be used to let model text decide whether to call GitHub directly; code-owned adapters perform API calls and validation.

## Review Algorithm

1. Normalize webhook payloads into `review.Request`.
2. Validate the webhook signature and event/action before trusting repository, PR, or sender fields.
3. Enrich with Pulls, Issues, Checks, and Commits API data before corpus selection or model review.
4. Fetch files and patches with pagination; normalize renamed, removed, binary, generated, and large files explicitly.
5. Fetch comments, reviews, labels, requested reviewers, checks, and commit statuses to avoid repeating known information.
6. Preserve stable IDs, URLs, head/base SHAs, and commit refs for traceability and idempotent publishing.
7. Publish only after deterministic finding validation and HIL/policy gates approve the report.
8. Re-read existing bot comments immediately before publishing to avoid duplicate reports in concurrent runs.

## Required API Surface

- Pull request metadata: `GET /repos/{owner}/{repo}/pulls/{pull_number}`
- Pull request commits: `GET /repos/{owner}/{repo}/pulls/{pull_number}/commits`
- Pull request files: `GET /repos/{owner}/{repo}/pulls/{pull_number}/files`
- Issue comments: `GET /repos/{owner}/{repo}/issues/{issue_number}/comments`
- Review comments: `GET /repos/{owner}/{repo}/pulls/{pull_number}/comments`
- Reviews: `GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews`
- Check runs: `GET /repos/{owner}/{repo}/commits/{ref}/check-runs`
- Commit statuses: `GET /repos/{owner}/{repo}/commits/{ref}/status`
- Draft/final issue comment: `POST /repos/{owner}/{repo}/issues/{issue_number}/comments`
- Inline review comment: prefer review creation APIs when commenting on diff lines.

## Technical Patterns

### Webhook Trust Boundary

- Verify `X-Hub-Signature-256` using the configured webhook secret before decoding business fields.
- Accept only expected events and actions. For pull request workflows, unsupported actions should become ignored events, not partial reviews.
- Treat webhook file lists as hints only; enrich from the API because payloads can be incomplete or stale.
- Store delivery IDs and event timestamps for deduplication and diagnostics.

### Pagination and Rate Limits

- Page all list endpoints until exhaustion. Do not assume default page size is complete.
- Respect `Link` headers and secondary rate-limit responses.
- Retry only idempotent reads automatically; publishing retries must be guarded by idempotency markers.
- Surface partial-enrichment failures in run metadata so the report can show degraded context instead of pretending completeness.

### Diff and Comment Positioning

- Normalize GitHub file statuses into common review statuses: added, modified, deleted, renamed, binary, too-large, generated.
- Keep `filename`, `previous_filename`, `patch`, additions, deletions, and blob URLs where available.
- Inline comments require a valid commit and diff position/line contract. If the exact line is unavailable after rebases or large diffs, fall back to a summary comment that cites the file path.
- Never invent line positions from model output; deterministic diff code owns placement.

### Authentication and Permissions

- Prefer GitHub App installation tokens for production deployments; use least-privilege fine-grained tokens only where appropriate.
- Required permissions commonly include pull request read/write, contents read, checks read, metadata read, and issues write for PR summary comments.
- Do not log tokens, authorization headers, signed webhook payloads, or full private diffs.

## Normalization Rules

Map GitHub PRs to internal review vocabulary:

- repository owner/name -> `review.Request.Repository`
- pull request number -> `review.Request.ChangeID`
- `base.sha` -> target SHA
- `head.sha` -> source SHA
- `user.login` -> author
- labels -> review labels
- files -> normalized changed files with filename, patch, additions, deletions, status

Always page through list endpoints until exhausted. Preserve GitHub node IDs and URLs in metadata for publishing and idempotency.

## Adapter Contract

GitHub logic belongs in deterministic code, not prompt text. The skill should cause the agent to verify that these code-owned responsibilities exist:

- Webhook handler: validates `X-Hub-Signature-256`, checks `X-GitHub-Event`, filters PR actions, captures delivery ID, and emits a normalized `review.Request`.
- Enricher: reads PR metadata, commits, files, labels, comments, reviews, checks, and statuses through paginated API calls.
- Diff normalizer: converts GitHub file records into provider-neutral changed files without losing rename, binary, or truncation state.
- Publisher: searches for stable review-bot markers, updates or supersedes existing comments, and never posts directly from model output.
- Audit trail: records endpoint failures, partial context, delivery IDs, report IDs, and publish URLs in run state.

## Failure Modes To Prioritize

- A forged webhook can trigger a review because signature validation is absent or happens after trusted fields are consumed.
- Large PRs are reviewed partially because files, commits, comments, checks, or reviews are not paginated.
- Rebased PRs get inline comments on invalid lines because model-provided positions are trusted.
- Draft and final reports collide because marker kind, project, or change ID is not part of the idempotency key.
- GitHub App or token scopes are broader than needed, or missing scopes degrade publishing without an explicit operator-visible error.

## Test Expectations

- Valid and invalid signature tests using representative payload bytes.
- Unsupported event/action tests that return ignored/accepted status without enqueueing review work.
- Multi-page files, commits, and comments tests with `Link` header pagination.
- Publish idempotency tests for existing draft and final markers.
- Permission/error tests that prove partial enrichment is recorded rather than hidden.

## Execution Rules

GitHub findings should identify which adapter obligation failed: webhook trust,
API enrichment, diff normalization, provider metadata preservation, or
publishing idempotency. File when the deterministic GitHub adapter would enqueue
untrusted work, lose PR context through pagination or payload assumptions,
produce provider-neutral state that cannot be published safely, or post duplicate
bot comments. Suppress model-level review complaints unless the adapter evidence
itself is wrong or missing.

Choose the fix location:

- webhook parser for signature, event, action, delivery, or repo identity issues
- API client for pagination, auth, timeout, status, or schema handling
- diff normalizer for paths, rename/delete/binary/truncation, or line mapping
- publisher for marker search, update semantics, final/draft separation, or URLs
- run state for audit fields needed by HIL and follow-up chat

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Webhook normalization | `scm-api` | Validate HMAC, parse pull request event fields, delivery ID, repository identity, labels, branches, and changed path hints. |
| Enrichment | `scm-api`, `diff-analyzer` | Fetch PR metadata, commits, files, checks/statuses, comments, and review threads with pagination. |
| Publishing | `publisher` | Use draft/final bot markers, update prior comments idempotently, and preserve provider IDs or URLs. |
| Validation | `validator` | Reject adapter findings that lack request, endpoint, pagination, auth, or idempotency evidence. |

## Escalation Signals

- HMAC validation, delivery dedupe, repository fallback, pagination, or marker updates are skipped.
- GitHub PR files/comments/checks are assumed from webhook payload alone.
- Draft and final reports can overwrite each other or duplicate bot comments.

## Evidence Standard

Provider findings should cite the specific missing field, endpoint behavior, pagination path, or publishing contract. For example: "files endpoint is read once without following pagination", "webhook parser trusts payload before signature validation", or "publisher posts a new issue comment without searching for the review-bot marker".

## Runtime Integration Checks

- Webhook handling must validate signature before parsing trusted fields and must preserve delivery ID, repository, PR number, branch refs, labels, and changed paths in normalized request state.
- Enrichment must follow pagination for files, commits, comments, checks/statuses, and reviews; partial API failure must be represented in run context or fail explicitly.
- Publishing must search for existing draft/final markers before posting and store provider comment IDs or URLs for later finalization.
- GitHub-specific structures must not leak past normalization except inside SCM context and publisher metadata.

## Review Output Contract

Report GitHub integration issues with the endpoint/header/event, the normalized field or side effect that becomes wrong, and the exact fake-server test needed. Distinguish forged webhook risk, missed context risk, duplicate publishing risk, and permission/scope risk.

## False Positive Checks

- Do not require inline comments when the finding is repository-wide, generated, binary, or outside available diff hunks.
- Do not block a run for unavailable checks/statuses if the API returns an authorized empty set and the report records that context.
- Do not require GraphQL when the REST API supplies the needed fields consistently.
- Do not recommend broad token scopes when a narrower GitHub App permission set satisfies the behavior.

## Review Safety

Do not publish directly from LLM output. Route findings through validation, report rendering, and HIL/policy checks. Before posting, search prior bot comments to update or supersede them instead of duplicating reports.

## Finding Template

```md
### [severity] GitHub integration issue

- API boundary: `<webhook/enrichment/pagination/publishing/auth>`
- Endpoint or header: `<specific GitHub API/header/event>`
- Problem: `<failure mode>`
- Evidence: `<changed lines or missing handling>`
- Production impact: `<missed files, unsafe publish, duplicate comment, stale context, auth risk>`
- Suggested fix: `<deterministic adapter behavior and test>`
```

Official docs:

- https://docs.github.com/en/rest/pulls/pulls
- https://docs.github.com/en/rest/pulls/comments
