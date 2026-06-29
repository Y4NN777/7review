---
sidebar_position: 3
title: Configuration
description: Required environment variables and review policy settings.
---

# Configuration

Generate `.env` with the setup command:

```bash
go run ./cmd/7review setup
```

## Required variables

```bash
LISTEN_ADDR=:8080
REVIEW_API_TOKEN=change-me
ORCHESTRATOR_CONFIG=./orchestrator.yaml
HEADROOM_URL=http://headroom:8787
MEMPALACE_URL=http://mempalace:8788
MEMORY_DIR=./.7review
CORPUS_ROOT=.
WEBHOOK_WORKERS=4
WEBHOOK_QUEUE_SIZE=128
WEBHOOK_JOB_TIMEOUT_MS=900000
```

## GitHub

```bash
GITHUB_API_URL=https://api.github.com
GITHUB_TOKEN=...
GITHUB_WEBHOOK_SECRET=...
```

## GitLab

```bash
GITLAB_URL=https://gitlab.com
GITLAB_TOKEN=...
GITLAB_WEBHOOK_SECRET=...
```

## Review policy

```bash
WEBHOOK_REVIEW_MODE=manual_first
REVIEW_LABEL_INCLUDE=7review,ready-for-review
REVIEW_LABEL_EXCLUDE=no-review,wip,draft
REVIEW_ALLOWED_PROJECTS=
REVIEW_ALLOWED_REPOS=
REVIEW_BRANCH_INCLUDE=
REVIEW_BRANCH_EXCLUDE=
```

## Model providers

The orchestrator can route work through OpenAI, Anthropic, OpenRouter, DeepSeek,
Mistral, Gemini, Ollama, or OpenAI-compatible endpoints. Use
`ORCHESTRATOR_CONFIG=./orchestrator.yaml` for role routing, or set a single
fallback provider with `PROVIDER`, `PROVIDER_API_KEY`, `REVIEW_MODEL`, and
`SMALL_MODEL`.

## Security checks

- Do not commit `.env`.
- Keep `orchestrator.yaml` free of secrets.
- Use `REVIEW_API_TOKEN` for operator routes.
- Use provider-specific webhook secrets for webhook routes.
