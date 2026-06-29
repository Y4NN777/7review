---
sidebar_position: 6
title: Déploiement Docker
description: Exécuter 7review, Headroom et MemPalace avec Docker Compose.
---

# Déploiement Docker

La stack Compose lance :

- `7review` : serveur webhook Go et pipeline review
- `headroom` : sidecar de réduction de contexte
- `mempalace` : sidecar de mémoire durable

```bash
HEADROOM_URL=http://headroom:8787
MEMPALACE_URL=http://mempalace:8788
```

## Démarrer

```bash
go run ./cmd/7review setup
make docker-up
make docker-status
```

## Targets Make

```bash
make docker-ready
make docker-review-gitlab PROJECT_ID=25 MR=19
make docker-review-github REPO=owner/repo PR=7
make docker-logs
make docker-tui
make docker-down
```
