---
sidebar_position: 7
title: Operator CLI and TUI
description: Commands for status, sessions, chat, approval, and final publishing.
---

# Operator CLI and TUI

Operator routes require `REVIEW_API_TOKEN` through `Authorization: Bearer` or
`X-7review-Token`.

## Status

```bash
7review status
7review status --server http://localhost:8080
```

Remote status calls `/ready` and reports pipeline, queue, run store, Headroom,
MemPalace, and orchestrator state.

## Sessions

```bash
7review sessions --server http://localhost:8080
7review sessions --status drafted --provider github --limit 10 --server http://localhost:8080
7review session owner/repo!7 --server http://localhost:8080
7review history owner/repo!7 --server http://localhost:8080
```

## Chat and TUI

```bash
7review chat owner/repo!7 --server http://localhost:8080
7review tui --server http://localhost:8080
7review tui --once --server http://localhost:8080
```

Inside chat or TUI, use slash commands:

```text
/status
/sessions
/run
/diff
/history
/draft
/memory
/approve --report-file final.md
/publish-final --report-file final.md
```

## HIL actions

Final publication is explicit:

```bash
7review approve --run owner/repo!7 --report-file final.md --server http://localhost:8080
7review publish-final --run owner/repo!7 --report-file final.md --server http://localhost:8080
```
