---
name: gitlab-merge-api
description: Master GitLab merge request APIs for review enrichment and publishing. Use when the agent needs to normalize GitLab webhook events, fetch MR metadata, commits, diffs, pipelines, discussions, labels, approvals, or publish draft/final merge request notes.
---

# GitLab Merge API Skill

## Workflow

1. Normalize GitLab webhook payloads into `review.Request`.
2. Enrich from GitLab Merge Requests API before review.
3. Fetch diffs, commits, discussions, notes, approvals, and pipelines.
4. Build a normalized review source independent of GitLab naming.
5. Publish draft/final review notes only through the publisher after validation and HIL policy.

## Required API Surface

- MR metadata: `GET /projects/:id/merge_requests/:merge_request_iid`
- MR commits: `GET /projects/:id/merge_requests/:merge_request_iid/commits`
- MR changes/diffs: use GitLab MR diff endpoints available for the configured GitLab version.
- MR pipelines: `GET /projects/:id/merge_requests/:merge_request_iid/pipelines`
- MR discussions: `GET /projects/:id/merge_requests/:merge_request_iid/discussions`
- MR notes: `GET /projects/:id/merge_requests/:merge_request_iid/notes`
- Publish note: `POST /projects/:id/merge_requests/:merge_request_iid/notes`
- Publish threaded discussion when line-specific comments are required.

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

## Review Safety

Never assume webhook payloads contain enough context. Always enrich before review. Do not publish duplicate bot notes; find previous bot-authored notes and update, supersede, or append according to publisher policy.

Official docs:

- https://docs.gitlab.com/api/merge_requests/
- https://docs.gitlab.com/api/discussions/

