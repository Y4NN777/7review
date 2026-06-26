---
name: github-merge-api
description: Master GitHub pull request APIs for review enrichment and publishing. Use when the agent needs to normalize GitHub webhook events, fetch PR metadata, commits, changed files, checks, discussions, labels, reviewers, or publish draft/final review comments.
---

# GitHub Merge API Skill

## Workflow

1. Normalize webhook payloads into `review.Request`.
2. Enrich with GitHub REST data before review.
3. Fetch files and patches before project knowledge selection.
4. Fetch comments, reviews, labels, checks, and commit statuses for live review context.
5. Publish comments through the GitHub API only after validation and HIL policy allow it.

## Required API Surface

- Pull request metadata: `GET /repos/{owner}/{repo}/pulls/{pull_number}`
- Pull request commits: `GET /repos/{owner}/{repo}/pulls/{pull_number}/commits`
- Pull request files: `GET /repos/{owner}/{repo}/pulls/{pull_number}/files`
- Issue comments: `GET /repos/{owner}/{repo}/issues/{issue_number}/comments`
- Review comments: `GET /repos/{owner}/{repo}/pulls/{pull_number}/comments`
- Reviews: `GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews`
- Check runs: `GET /repos/{owner}/{repo}/commits/{ref}/check-runs`
- Commit statuses: `GET /repos/{owner}/{repo}/commits/{ref}/status`
- Draft/final issue comment: `POST /repos/{owner}/{repo}/issues/{issue_number}/comments`
- Inline review comment: prefer review creation APIs when commenting on diff lines.

## Normalization Rules

Map GitHub PRs to internal review vocabulary:

- repository owner/name -> `review.Request.Repository`
- pull request number -> `review.Request.ChangeID`
- `base.sha` -> target SHA
- `head.sha` -> source SHA
- `user.login` -> author
- labels -> review labels
- files -> normalized changed files with filename, patch, additions, deletions, status

Always page through list endpoints until exhausted. Preserve GitHub node IDs and URLs in metadata for publishing and idempotency.

## Review Safety

Do not publish directly from LLM output. Route findings through validation, report rendering, and HIL/policy checks. Before posting, search prior bot comments to update or supersede them instead of duplicating reports.

Official docs:

- https://docs.github.com/en/rest/pulls/pulls
- https://docs.github.com/en/rest/pulls/comments

