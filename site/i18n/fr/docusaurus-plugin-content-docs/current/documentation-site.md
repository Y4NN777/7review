---
sidebar_position: 7
title: Site de documentation
description: Builder et déployer le site Docusaurus.
---

# Site de documentation

Le site de documentation vit dans `site/`. C’est un site Docusaurus avec contenu
anglais et français, la mascotte 7review et un workflow GitHub Pages.

## Commandes locales

Installer les dépendances une fois :

```bash
cd site
npm install
```

Lancer le serveur de développement :

```bash
npm run start -- --port 3000 --host 0.0.0.0
```

Builder le site de production :

```bash
npm run build
```

Servir le build généré localement :

```bash
npm run serve -- --host 0.0.0.0
```

## Configuration GitHub Pages

Le workflow `.github/workflows/pages.yml` build `site/` et déploie l’artefact
`site/build` avec `actions/deploy-pages`.

GitHub demande un réglage du dépôt avant le premier déploiement :

1. Ouvre le dépôt sur GitHub.
2. Va dans **Settings**.
3. Ouvre **Pages**.
4. Dans **Build and deployment**, mets **Source** sur **GitHub Actions**.
5. Sauvegarde, puis relance le workflow `Deploy documentation`.

Le workflow n’active pas Pages automatiquement, parce que le `GITHUB_TOKEN` par
défaut peut déployer vers Pages, mais ne peut pas toujours créer ou activer le
site Pages du dépôt. Quand Pages n’est pas activé, GitHub renvoie
`Resource not accessible by integration`.

## Chemin publié

Le `baseUrl` Docusaurus est configuré pour :

```text
/7review/
```

Pour un dépôt nommé `7review`, le site publié doit être disponible sous :

```text
https://<owner>.github.io/7review/
```

## Si le workflow échoue

| Symptôme | Cause | Correction |
| --- | --- | --- |
| `Resource not accessible by integration` | Pages n’a pas été activé depuis les paramètres du dépôt | Mettre la source Pages sur GitHub Actions, puis relancer |
| `Missing script: start` | La commande a été lancée hors de `site/` | Faire `cd site` d’abord |
| Le build passe mais les styles ou routes sont faux | `baseUrl` ne correspond pas au chemin du dépôt | Vérifier `baseUrl` dans `site/docusaurus.config.js` |
