---
name: review-publisher
description: Publish draft and final review reports safely to GitHub or GitLab. Use when the agent needs to format findings, avoid duplicate bot comments, create inline comments, update previous reports, or apply human-in-the-loop approval policy before publishing.
---

# Review Publisher Skill

## Workflow

1. Render validated findings into deterministic Markdown.
2. Search previous bot comments before posting.
3. Publish draft reports only when configured for draft/HIL mode.
4. Publish final reports only after approval or when policy allows auto-final.
5. Include traceability citations and model/provider metadata.

## Idempotency

Use a stable marker in bot comments:

```md
<!-- 7review:project=<project-id>;change=<change-id>;kind=draft -->
```

Before publishing:

- find existing marker for the same project/change/kind
- update if the SCM API supports it
- otherwise add a superseding comment and mark the old one stale

## Report Sections

- Summary
- Blocking findings
- Non-blocking findings
- Traceability gaps
- Security/design/API contract risks
- Validation notes
- Sources used
- Provider/model footer

Do not include raw secrets, full private diffs beyond necessary snippets, or unvalidated model output.

