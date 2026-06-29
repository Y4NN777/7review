---
sidebar_position: 5
title: Webhook Policy
description: Control which webhook events enqueue review work.
---

# Webhook Policy

Webhook review is policy-gated. A valid webhook payload can be accepted and
still not enqueue review work. This keeps the default operating model manual
first while preserving webhooks for teams that want controlled automation.

## Modes

| Mode | Behavior |
| --- | --- |
| `manual_first` | Default. Enqueue only when include policy matches and no exclusion rejects the event. |
| `auto` | Enqueue by default, but explicit excludes and allowlists still apply. |
| `off` | Validate and accept valid webhook payloads, but never enqueue review work. |

Use `manual_first` when operators should choose normal reviews and labels should
opt selected changes into automation. Use `auto` when the repository is ready
for broad automated review. Use `off` when you only want signature validation
and observability.

## Environment

```bash
WEBHOOK_REVIEW_MODE=manual_first
REVIEW_LABEL_INCLUDE=7review,ready-for-review
REVIEW_LABEL_EXCLUDE=no-review,wip,draft
REVIEW_ALLOWED_PROJECTS=
REVIEW_ALLOWED_REPOS=
REVIEW_BRANCH_INCLUDE=
REVIEW_BRANCH_EXCLUDE=
```

Lists are comma-separated and trimmed. Empty allowlists mean "allow any" for
that dimension.

## Decision order

7review evaluates webhook policy as a gate before the worker queue:

1. Validate the provider signature and parse the webhook payload.
2. Reject automation when `WEBHOOK_REVIEW_MODE=off`.
3. Apply project or repository allowlists.
4. Apply branch include and exclude lists.
5. Apply label excludes.
6. In `manual_first`, require at least one include label.
7. If allowed, enqueue the normalized review request.

Excludes win over includes. A PR with both `7review` and `no-review` is ignored.

## Label policy

```bash
REVIEW_LABEL_INCLUDE=7review,ready-for-review
REVIEW_LABEL_EXCLUDE=no-review,wip,draft
```

In `manual_first`, a PR or MR needs one include label to enqueue review work.
In `auto`, include labels are not required, but exclude labels still block
automation.

## Project and repository allowlists

Use allowlists when one 7review instance receives events from multiple sources:

```bash
REVIEW_ALLOWED_PROJECTS=25,26
REVIEW_ALLOWED_REPOS=owner/repo,org/app
```

GitLab events are matched against project IDs. GitHub events are matched against
`owner/repo`.

## Branch policy

Branch filters let you keep automation on stable integration branches while
ignoring scratch work:

```bash
REVIEW_BRANCH_INCLUDE=main,release
REVIEW_BRANCH_EXCLUDE=wip,experiment
```

If an include list is set, the branch must match it. If an exclude list matches,
the event is ignored even when another rule would allow it.

## Responses and observability

Ignored webhook events return `202 Accepted` when the signature and payload are
valid. The response explains that the event was ignored by review policy rather
than treating it as an error.

Use readiness and status commands to confirm the active mode and queue state:

```bash
7review status --server http://localhost:8080
curl -fsS -H "Authorization: Bearer $REVIEW_API_TOKEN" http://localhost:8080/ready
```

## Common configurations

| Goal | Configuration |
| --- | --- |
| Manual-first with opt-in label | `WEBHOOK_REVIEW_MODE=manual_first` and `REVIEW_LABEL_INCLUDE=7review` |
| Review every accepted webhook except drafts | `WEBHOOK_REVIEW_MODE=auto` and `REVIEW_LABEL_EXCLUDE=draft,wip,no-review` |
| Disable webhook review but keep validation | `WEBHOOK_REVIEW_MODE=off` |
| Limit automation to one repo | `REVIEW_ALLOWED_REPOS=owner/repo` |
| Limit automation to release work | `REVIEW_BRANCH_INCLUDE=main,release` |
