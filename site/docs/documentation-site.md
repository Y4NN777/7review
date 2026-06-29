---
sidebar_position: 7
title: Documentation Site
description: Build and deploy the Docusaurus documentation site.
---

# Documentation Site

The documentation site lives in `site/`. It is a Docusaurus site with English
and French content, the 7review mascot, and a GitHub Pages workflow.

## Local commands

Install dependencies once:

```bash
cd site
npm install
```

Run the development server:

```bash
npm run start -- --port 3000 --host 0.0.0.0
```

Build the production site:

```bash
npm run build
```

Serve the generated build locally:

```bash
npm run serve -- --host 0.0.0.0
```

## GitHub Pages setup

The workflow at `.github/workflows/pages.yml` builds `site/` and deploys the
generated `site/build` artifact with `actions/deploy-pages`.

GitHub requires one repository setting before the first deployment:

1. Open the repository on GitHub.
2. Go to **Settings**.
3. Open **Pages**.
4. Under **Build and deployment**, set **Source** to **GitHub Actions**.
5. Save the setting, then rerun the `Deploy documentation` workflow.

The workflow does not enable Pages automatically because the default
`GITHUB_TOKEN` can deploy to Pages, but it cannot always create or enable the
Pages site for the repository. When Pages is not enabled, GitHub returns
`Resource not accessible by integration`.

## Published path

The Docusaurus base URL is configured for:

```text
/7review/
```

For a repository named `7review`, the published site should be available under:

```text
https://<owner>.github.io/7review/
```

## When the workflow fails

| Symptom | Cause | Fix |
| --- | --- | --- |
| `Resource not accessible by integration` | Pages has not been enabled from repository settings | Set Pages source to GitHub Actions, then rerun |
| `Missing script: start` | Command was run outside `site/` | Run `cd site` first |
| Build succeeds but styles or routes are wrong | `baseUrl` does not match the repository path | Check `baseUrl` in `site/docusaurus.config.js` |
