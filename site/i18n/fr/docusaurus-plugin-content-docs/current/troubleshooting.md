---
sidebar_position: 10
title: Diagnostic
description: Diagnostiquer les erreurs opérateur courantes.
---

# Diagnostic

Commence par la readiness :

```bash
7review status --server http://localhost:8080
```

## Webhook accepté mais aucune review

En `manual_first`, un webhook valide sans label d'inclusion est accepté puis
ignoré par politique.

```bash
WEBHOOK_REVIEW_MODE=manual_first
REVIEW_LABEL_INCLUDE=7review,ready-for-review
REVIEW_LABEL_EXCLUDE=no-review,wip,draft
```

## Review manuelle rejetée

Causes fréquentes :

- même run déjà en file ou en cours
- file de travail pleine
- provider incomplet
- token opérateur absent

```bash
7review sessions --server http://localhost:8080
7review status --server http://localhost:8080
```

## Brouillon publié mais pas de final

C'est normal sans approbation HIL :

```bash
7review approve --run owner/repo!7 --report-file final.md --server http://localhost:8080
```
