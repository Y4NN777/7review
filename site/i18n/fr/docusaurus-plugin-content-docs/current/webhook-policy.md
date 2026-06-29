---
sidebar_position: 5
title: Politique webhook
description: Contrôler quels webhooks lancent une review.
---

# Politique webhook

Les webhooks sont filtrés par politique. Un payload valide peut être accepté
sans lancer de review.

## Modes

| Mode | Comportement |
| --- | --- |
| `manual_first` | Défaut. Lance seulement si la politique d'inclusion correspond. |
| `auto` | Lance sauf exclusion explicite ou allowlist non respectée. |
| `off` | Accepte les webhooks valides mais ne lance aucune review. |

## Variables

```bash
WEBHOOK_REVIEW_MODE=manual_first
REVIEW_LABEL_INCLUDE=7review,ready-for-review
REVIEW_LABEL_EXCLUDE=no-review,wip,draft
REVIEW_ALLOWED_PROJECTS=
REVIEW_ALLOWED_REPOS=
REVIEW_BRANCH_INCLUDE=
REVIEW_BRANCH_EXCLUDE=
```

`REVIEW_LABEL_EXCLUDE` gagne toujours sur les labels d'inclusion.
