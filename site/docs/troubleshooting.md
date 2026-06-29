---
sidebar_position: 10
title: Troubleshooting
description: Diagnose common operator failures.
---

# Troubleshooting

Start with readiness:

```bash
7review status --server http://localhost:8080
```

## Webhook accepted but no review started

Check policy:

```bash
WEBHOOK_REVIEW_MODE=manual_first
REVIEW_LABEL_INCLUDE=7review,ready-for-review
REVIEW_LABEL_EXCLUDE=no-review,wip,draft
```

In `manual_first`, a valid webhook without an include label is accepted and
ignored by policy. Trigger manually if you need an immediate review.

## Manual review rejected

Common causes:

- same run is already queued or running
- worker queue is full
- provider input is incomplete
- operator token is missing

Check sessions and queue:

```bash
7review sessions --server http://localhost:8080
7review status --server http://localhost:8080
```

## Sidecar down

`/ready` reports `headroom` or `mempalace` failures. In Docker, verify service
DNS names:

```bash
HEADROOM_URL=http://headroom:8787
MEMPALACE_URL=http://mempalace:8788
```

## Draft published but final not published

This is expected until HIL approval. Approve with a reviewed report file:

```bash
7review approve --run owner/repo!7 --report-file final.md --server http://localhost:8080
```

Retry final publishing:

```bash
7review publish-final --run owner/repo!7 --report-file final.md --server http://localhost:8080
```
