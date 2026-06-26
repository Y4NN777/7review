---
name: traceability-review
description: Enforce traceability between code changes and product/design/security contracts. Use when review findings must connect changed files to FR, NFR, INV, GAR, PRO, LAW, OPC, ADR, CMP, DSO, CTRL, or threat-model references.
---

# Traceability Review Skill

## Workflow

1. Extract identifiers from PR/MR text, commits, changed files, and selected knowledge.
2. Map changes to requirements and architectural contracts.
3. Prefer findings that cite violated or missing traceability.
4. Reject findings that cannot connect behavior to code, specification, or risk.

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

## Review Questions

- Does the change preserve all relevant invariants?
- Does it implement or alter a requirement without citing the requirement?
- Does it contradict an ADR or component responsibility?
- Does it cross a VPS/LOCAL, privacy, crypto, identity, or moderation boundary?
- Does it update API behavior without updating OpenAPI/AsyncAPI or SRS?
- Does it touch UI/design without preserving design tokens or visual contract?

## Finding Shape

Every traceability finding should include:

- changed file/path
- violated or missing reference ID
- why the implementation conflicts with that reference
- concrete remediation
- confidence and severity

