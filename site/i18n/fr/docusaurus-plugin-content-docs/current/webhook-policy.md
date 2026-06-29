---
sidebar_position: 5
title: Politique webhook
description: Contrôler quels webhooks mettent une review en file.
---

# Politique webhook

Les reviews déclenchées par webhook sont filtrées par politique. Un payload
webhook valide peut être accepté sans mettre de review en file. Cela garde le
modèle d’exploitation par défaut en manuel d’abord, tout en conservant les
webhooks pour les équipes qui veulent une automatisation contrôlée.

## Modes

| Mode | Comportement |
| --- | --- |
| `manual_first` | Défaut. Met en file seulement si la politique d’inclusion correspond et qu’aucune exclusion ne bloque l’événement. |
| `auto` | Met en file par défaut, mais les exclusions explicites et les allowlists s’appliquent toujours. |
| `off` | Valide et accepte les payloads webhook valides, mais ne met jamais de review en file. |

Utilise `manual_first` quand les opérateurs doivent choisir les reviews
normales et que les labels servent à activer certaines automatisations. Utilise
`auto` quand le dépôt est prêt pour une review automatisée plus large. Utilise
`off` quand tu veux seulement la validation de signature et l’observabilité.

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

Les listes sont séparées par virgules et nettoyées des espaces. Une allowlist
vide signifie “autoriser tout” pour cette dimension.

## Ordre de décision

7review évalue la politique webhook comme une porte avant la file de workers :

1. Valider la signature provider et parser le payload webhook.
2. Refuser l’automatisation si `WEBHOOK_REVIEW_MODE=off`.
3. Appliquer les allowlists projet ou dépôt.
4. Appliquer les listes d’inclusion et d’exclusion de branches.
5. Appliquer les labels d’exclusion.
6. En `manual_first`, exiger au moins un label d’inclusion.
7. Si l’événement est autorisé, mettre en file la requête de review normalisée.

Les exclusions gagnent sur les inclusions. Une PR avec `7review` et `no-review`
est ignorée.

## Politique de labels

```bash
REVIEW_LABEL_INCLUDE=7review,ready-for-review
REVIEW_LABEL_EXCLUDE=no-review,wip,draft
```

En `manual_first`, une PR ou MR doit avoir au moins un label d’inclusion pour
mettre une review en file. En `auto`, les labels d’inclusion ne sont pas requis,
mais les labels d’exclusion bloquent toujours l’automatisation.

## Allowlists projet et dépôt

Utilise les allowlists quand une même instance 7review reçoit des événements de
plusieurs sources :

```bash
REVIEW_ALLOWED_PROJECTS=25,26
REVIEW_ALLOWED_REPOS=owner/repo,org/app
```

Les événements GitLab sont comparés aux IDs de projet. Les événements GitHub
sont comparés au format `owner/repo`.

## Politique de branches

Les filtres de branches permettent de garder l’automatisation sur les branches
d’intégration stables en ignorant le travail temporaire :

```bash
REVIEW_BRANCH_INCLUDE=main,release
REVIEW_BRANCH_EXCLUDE=wip,experiment
```

Si une liste d’inclusion est définie, la branche doit correspondre. Si une liste
d’exclusion correspond, l’événement est ignoré même si une autre règle
l’autorise.

## Réponses et observabilité

Les événements webhook ignorés renvoient `202 Accepted` quand la signature et le
payload sont valides. La réponse explique que l’événement a été ignoré par la
politique de review au lieu de le traiter comme une erreur.

Utilise les commandes readiness et status pour confirmer le mode actif et l’état
de la file :

```bash
7review status --server http://localhost:8080
curl -fsS -H "Authorization: Bearer $REVIEW_API_TOKEN" http://localhost:8080/ready
```

## Configurations courantes

| Objectif | Configuration |
| --- | --- |
| Manuel d’abord avec label opt-in | `WEBHOOK_REVIEW_MODE=manual_first` et `REVIEW_LABEL_INCLUDE=7review` |
| Reviewer chaque webhook accepté sauf les drafts | `WEBHOOK_REVIEW_MODE=auto` et `REVIEW_LABEL_EXCLUDE=draft,wip,no-review` |
| Désactiver la review webhook mais garder la validation | `WEBHOOK_REVIEW_MODE=off` |
| Limiter l’automatisation à un dépôt | `REVIEW_ALLOWED_REPOS=owner/repo` |
| Limiter l’automatisation au travail release | `REVIEW_BRANCH_INCLUDE=main,release` |
