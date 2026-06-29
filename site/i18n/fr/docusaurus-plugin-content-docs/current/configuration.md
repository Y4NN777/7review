---
sidebar_position: 3
title: Configuration
description: Variables d'environnement requises et politique de review.
---

# Configuration

Génère `.env` avec :

```bash
go run ./cmd/7review setup
```

## Variables communes

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

## Politique webhook

```bash
WEBHOOK_REVIEW_MODE=manual_first
REVIEW_LABEL_INCLUDE=7review,ready-for-review
REVIEW_LABEL_EXCLUDE=no-review,wip,draft
REVIEW_ALLOWED_PROJECTS=
REVIEW_ALLOWED_REPOS=
REVIEW_BRANCH_INCLUDE=
REVIEW_BRANCH_EXCLUDE=
```

## Sécurité

- Ne commit pas `.env`.
- Garde `orchestrator.yaml` sans secrets.
- Utilise `REVIEW_API_TOKEN` pour les routes opérateur.
- Utilise les secrets webhook GitHub/GitLab pour les routes webhook.
