---
name: traceability-review
description: Use when code changes must be traced to requirements, invariants, architecture decisions, threat controls, API contracts, process gates, or design sources. Enforces links between changed files and identifiers such as FR, NFR, INV, GAR, PRO, LAW, OPC, ADR, CMP, DSO, CTRL, and threat-model controls.
license: Apache-2.0
compatibility: Repositories with requirements, ADRs, contracts, threat models, SRS/PRD, API specs, or generated wiki context packs
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: traceability
  risk-tier: high
---

# Traceability Review Skill

## Activation Contract

Activate when changed behavior needs a link to requirements, invariants,
architecture decisions, API contracts, threat controls, design sources, delivery
rules, or generated wiki context. Use this skill when identifiers appear in
PR/MR text, commits, docs, tests, filenames, comments, or selected corpus.

Do not use it to demand bureaucratic references for purely mechanical edits
unless the missing link creates a real review, audit, or maintenance risk.

## Workflow

1. Extract identifiers from PR/MR text, commits, changed files, and selected knowledge.
2. Map changes to the SWE Basics chain: PRD intent, SRS rule, system contract,
   invariant/constraint, responsibility owner, architecture/modeling source,
   implementation, and test.
3. Prefer findings that cite violated or missing traceability.
4. Reject findings that cannot connect behavior to code, specification, or risk.
5. Distinguish missing traceability from actual contract violation.

## Identifier Families

- `FR-*`: functional requirement
- `NFR-*`: non-functional requirement
- `INV-*`: invariant
- `GAR-*`: guarantee
- `PRO-*`: prohibition
- `LAW-*`: conflict-resolution law
- `OPC-*`: operation contract
- `ADR-*`: architecture decision
- `CMP-*`: logical component
- `DSO-*`: open security decision
- `CTRL-*`: threat-model control
- `TM-*`: threat model scenario
- `RISK-*`: documented risk or risk acceptance
- `DOD-*`: delivery/Definition-of-Done item
- `PRD-*`: product intent, use case, out-of-scope, or success criterion
- `SRS-*`: verifiable software requirement, rule, constraint, or error case
- `UC-*`: use case
- `ACT-*`: actor

## Extraction Algorithm

Search in:

- PR/MR title, description, labels, and commits
- changed file paths and package names
- selected corpus headings and context pack IDs
- comments and discussions from SCM enrichment
- test names and fixture names

Normalize identifiers case-insensitively, but preserve original spelling in the
finding. If an identifier appears in code but not in selected knowledge, report
the missing source only when it blocks review confidence.

## Traceability Matrix

Build a small mental matrix before filing:

```text
changed path -> behavior changed -> PRD/SRS/contract ID -> owner/component -> test/evidence
```

A valid traceability finding needs at least three columns filled. If only the
source ID is missing but behavior is clearly safe, prefer a warning or note.

## SWE Basics Trace Chain

For substantial behavior changes, try to trace the full chain:

```text
problem/use case -> PRD scope -> SRS rule -> system guarantee/prohibition
-> invariant/constraint -> responsible component -> implementation -> test
```

Do not require every repository to use these exact labels. Accept equivalent
local names, headings, ADRs, issue IDs, diagrams, acceptance criteria, or tests.
File a finding only when the missing link makes the change ambiguous, unsafe, or
hard to maintain.

## Review Questions

- Does the change preserve all relevant invariants?
- Does it implement or alter a requirement without citing the requirement?
- Does it contradict an ADR or component responsibility?
- Does it implement a use case that is outside PRD scope?
- Does it weaken a verifiable SRS `MUST` or `MUST NOT` rule?
- Does the rule live in the component assigned by the design/architecture source?
- Does it cross a VPS/LOCAL, privacy, crypto, identity, or moderation boundary?
- Does it update API behavior without updating OpenAPI/AsyncAPI or SRS?
- Does it touch UI/design without preserving design tokens or visual contract?
- Does it remove tests that were the only executable trace for a requirement?
- Does it alter memory, publish, or HIL behavior without linking to process rules?

## Finding Shape

Every traceability finding should include:

- changed file/path
- violated or missing reference ID
- why the implementation conflicts with that reference
- concrete remediation
- confidence and severity

## Execution Rules

Traceability findings require bidirectional evidence. File when a changed
behavior should map to a requirement, invariant, ADR, guard, control, or test and
the link is missing, stale, or contradictory. Suppress if the identifier is only
mentioned incidentally, the change is below the requirement boundary, or another
current source explicitly supersedes the old ID.

Choose the traceability action:

- code/test link when implementation changed but requirement coverage did not
- PRD/SRS/spec link when a requirement changed but implementation did not
- matrix repair when source, code, and test exist but references disagree
- uncertainty note when no authoritative ID exists for high-risk behavior
- citation repair when compressed or generated context dropped source identity

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Identifier extraction | `filesystem`, `corpus-selector` | Extract FR/NFR/INV/GAR/PRO/LAW/OPC/ADR/CMP/DSO/CTRL identifiers from docs, comments, tests, and selected corpus. |
| Change mapping | `diff-analyzer`, `scm-api` | Map changed paths and behavior to identifiers, source clauses, tests, and report findings. |
| Matrix validation | `validator` | Require bidirectional evidence before claiming missing requirement coverage or broken traceability. |

## Escalation Signals

- A change references requirement IDs but does not update code/tests/docs consistently.
- A high-risk behavior lacks any trace to source requirement, guard, ADR, or control.
- A finding cites an identifier without source path, clause, or changed behavior mapping.

## Evidence Standard

Traceability findings must connect at least three concrete facts: the changed
path or behavior, the source identifier or missing identifier, and the rule or
risk that makes the link necessary. If the requirement source cannot be found,
state the searched locations and keep severity proportional to the resulting
review uncertainty.

## Runtime Integration Checks

- Extract identifiers from request text, changed files, selected corpus, generated wiki context packs, durable memory recall, and tests, then deduplicate by source authority.
- Preserve identifier-to-source mapping through context compression and report rendering; compressed text without source identity is insufficient evidence.
- Validate bidirectional links: changed behavior should map to source requirements, and touched requirements should map to tests or implementation.
- Provider-specific IDs must not replace domain identifiers; MR/PR numbers are routing metadata, not requirements.

## Review Output Contract

Return a traceability matrix row for each issue: identifier, source path/heading, changed path, expected behavior, actual behavior, evidence confidence, and required remediation. For missing identifiers, include searched sources and why traceability is required.

For `confirmed` findings, include structured citations: `source`,
`heading_or_key`, `rule`, and `violation`. The `rule` must match selected
source-of-truth evidence, and `violation` must tie the changed path/line to the
broken trace. If the source clause is missing, output a note or question instead
of an inline finding.

## Severity Guidance

- critical/high: violated security control, data invariant, legal/privacy rule,
  HIL gate, or irreversible operation contract.
- medium: changed externally visible behavior lacks requirement/API trace or
  contradicts ADR/component responsibility.
- low/info: missing link or stale reference creates maintainability risk but no
  immediate behavior break.

## False Positive Checks

Do not report if:

- the PR/MR is intentionally documentation-only and updates the source of truth
- the identifier is unrelated text or a historical reference
- a more authoritative contract supersedes the stale reference
- the code path is deleted and no longer implements the requirement

## Finding Template

```text
Title: <traceability gap or contract violation>
Reference: <FR/NFR/INV/GAR/PRO/LAW/OPC/ADR/CMP/CTRL/etc>
Changed path: <file>
Expected: <what the reference requires>
Actual: <what the change does>
Risk: <why this matters>
Fix: <code, test, or contract update>
```
