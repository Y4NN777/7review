---
sidebar_position: 7
title: CLI et TUI opérateur
description: Commandes status, sessions, chat, approbation et publication finale.
---

# CLI et TUI opérateur

Les routes opérateur exigent `REVIEW_API_TOKEN`.

```bash
7review status --server http://localhost:8080
7review sessions --server http://localhost:8080
7review session owner/repo!7 --server http://localhost:8080
7review history owner/repo!7 --server http://localhost:8080
7review chat owner/repo!7 --server http://localhost:8080
7review tui --server http://localhost:8080
```

Commandes slash utiles :

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

La publication finale reste une action HIL explicite.
