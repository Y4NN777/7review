---
sidebar_position: 5
title: Webhook Policy
description: Control which webhook events enqueue review work.
---

# Webhook Policy

Webhook review is policy-gated. A valid webhook payload can be accepted and
still not enqueue review work.

## Modes

| Mode | Behavior |
| --- | --- |
| `manual_first` | Default. Enqueue only when include policy matches. |
| `auto` | Enqueue review unless explicit excludes or allowlists reject the event. |
| `off` | Accept valid webhook payloads, but do not enqueue review work. |

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

## Label policy

`REVIEW_LABEL_EXCLUDE` wins over include labels. A merge request or pull request
with `no-review`, `wip`, or `draft` is ignored even when it also has `7review`.

## Allowlist policy

Use project and repository allowlists when a single 7review instance receives
events from more than one source:

```bash
REVIEW_ALLOWED_PROJECTS=25,26
REVIEW_ALLOWED_REPOS=owner/repo,org/app
```

## Branch policy

Use branch include or exclude lists to limit automation by source or target
branch:

```bash
REVIEW_BRANCH_INCLUDE=main,release
REVIEW_BRANCH_EXCLUDE=wip
```

## Observability

Ignored webhook events return `202 Accepted` with an ignored reason when the
signature and payload are valid. Readiness and config tools show queue and
policy state:

```bash
7review status --server http://localhost:8080
curl -fsS -H "Authorization: Bearer $REVIEW_API_TOKEN" http://localhost:8080/ready
```
