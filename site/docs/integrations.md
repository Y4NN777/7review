---
sidebar_position: 9
title: Integrations
description: Headroom and MemPalace sidecar contracts.
---

# Integrations

Headroom and MemPalace are required production sidecars. They are external HTTP
services, not embedded Python or TypeScript code in the Go binary.

## Environment

```bash
HEADROOM_URL=http://headroom:8787
HEADROOM_TIMEOUT_MS=5000
MEMPALACE_URL=http://mempalace:8788
MEMPALACE_TIMEOUT_MS=5000
```

## Headroom

The pipeline sends assembled review context to Headroom for bounded context
reduction before model review.

## MemPalace

The pipeline uses MemPalace for:

- recall before review
- preview of approved memory proposals
- memory write after final human approval

## Readiness

`/ready` checks both sidecars:

```bash
curl -fsS \
  -H "Authorization: Bearer $REVIEW_API_TOKEN" \
  http://localhost:8080/ready
```
