---
sidebar_position: 6
title: Docker Deployment
description: Run 7review, Headroom, and MemPalace with Docker Compose.
---

# Docker Deployment

The Compose stack runs three services on one private network:

- `7review`: Go webhook server and review pipeline
- `headroom`: context reduction sidecar
- `mempalace`: durable memory sidecar

Only the agent publishes a host port. The Go service reaches sidecars through
service DNS:

```bash
HEADROOM_URL=http://headroom:8787
MEMPALACE_URL=http://mempalace:8788
```

## Start

```bash
go run ./cmd/7review setup
make docker-up
make docker-status
```

## Make targets

```bash
make docker-config
make docker-build
make docker-up
make docker-status
make docker-ready
make docker-review-gitlab PROJECT_ID=25 MR=19
make docker-review-github REPO=owner/repo PR=7
make docker-logs
make docker-tui
make docker-down
```

## Repository context

`CORPUS_ROOT` is the local repository or documentation tree that 7review should
scan for review context. Compose mounts it read-only at `/workspace`.

If `CORPUS_ROOT` is not set, Compose uses the current directory. For real
reviews, point it at the target repository checkout or a prepared context pack.

## Concurrency

```bash
WEBHOOK_WORKERS=4
WEBHOOK_QUEUE_SIZE=32
```

`WEBHOOK_WORKERS` controls review jobs. The reasoner role `max_parallel` in
`orchestrator.yaml` controls model fan-out inside one review.
