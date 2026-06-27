---
name: process-knowledge-review
description: Apply process knowledge such as Definition of Done, delivery rules, release gates, test expectations, operational playbooks, review policy, and team workflow constraints.
license: Apache-2.0
compatibility: "software delivery process documents, runbooks, release gates, review policy"
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator run-store
metadata:
  version: "2.0.0"
  owner: "7review"
  review-domain: "process-knowledge"
  risk-tier: "medium"
---

# Process Knowledge Review Skill

## Activation Contract

Use this skill when code correctness depends on process: tests, rollout, release sequencing, documentation, approvals, migrations, or operational safety.

Activate when selected corpus includes Definition of Done, release playbooks, incident response rules, rollout guides, team review agreements, operational runbooks, or compliance checklists. Also activate when the diff touches CI/CD, deployment scripts, migrations, observability, feature flags, or code paths that change engineer workflow.

## Review Algorithm

1. Identify the process artifacts selected for the review and their authority order.
2. Map the diff to delivery activities: build, test, deploy, migrate, monitor, rollback, approve, document.
3. Determine whether the change introduces a new operational or review obligation.
4. Check whether required artifacts changed with the code: tests, docs, runbook, config, migration notes, release checklist, alerting.
5. Compare risk tier against required approvals or HIL gates.
6. Report only missing process work that creates real production, maintenance, compliance, or delivery risk.
7. Prefer actionable gaps over generic "update docs" comments.

## Process Sources

- Definition of Done and review checklist
- release and deployment rules
- migration and rollback playbooks
- incident and observability requirements
- test strategy and coverage requirements
- HIL policy and approval workflow

## Technical Patterns

### Definition of Done

- Behavior changes should have tests or a clear reason tests are not useful.
- Public or operator-facing behavior should update docs, examples, or command help.
- Operational changes should include observability, rollout, and rollback implications.
- Security/privacy-sensitive changes should cite required reviews or approvals.

### Release and Rollout

- Feature flags need default behavior, cleanup plan, metrics, and compatibility with old/new versions.
- Database or memory-store changes need deploy ordering and rollback safety.
- API changes need consumer compatibility and communication path.
- Background jobs and queues need operational controls: retry, backoff, concurrency, idempotency, and dead-letter handling when applicable.

### Runbooks and Incidents

- New failure modes should have logs, metrics, alerts, or runbook instructions proportional to risk.
- Error messages should help operators distinguish configuration, provider, network, and validation failures.
- Manual recovery steps must not require hidden tribal knowledge.

### Review Policy

- High-risk domains may require security, data, compliance, or HIL approval.
- Human sign-off must be recorded before final publishing or durable memory when policy requires it.
- Exceptions should be explicit, dated, and scoped.

## Execution Rules

Process findings are justified by missing production evidence, not by preference.
File when the changed behavior creates an obligation that the repository process
requires and the diff does not add the matching artifact. Suppress when the
change is local-only, when another skill already reports the same blocking risk,
or when the process rule is obsolete and contradicted by newer instructions.

Pick the missing artifact explicitly:

- test evidence for changed executable behavior
- runbook/rollback evidence for deploy or recovery risk
- approval evidence for HIL, security, privacy, compliance, or final publish
- docs/help evidence for operator-facing CLI, TUI, API, or config changes

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Process source lookup | `filesystem`, `corpus-selector` | Locate Definition of Done, release playbooks, runbooks, review policy, migration notes, and operational docs. |
| Change mapping | `diff-analyzer`, `scm-api` | Map diff to tests, docs, rollout, rollback, approvals, observability, and deployment effects. |
| State/process cross-check | `run-store` | For HIL or review lifecycle process rules, verify durable run states and audit fields. |
| Validation | `validator` | Require both process source and changed behavior unless the operational risk is immediately evident. |

## Escalation Signals

- A production behavior changes without required tests, docs, rollout notes, rollback path, or approvals.
- A change introduces a new operator, admin, support, or developer workflow
  without command/API/UI documentation and tests.
- Process obligations are inferred loosely and cannot be tied to a durable source or convention.

## Evidence Standard

Process findings need two citations: the changed behavior and the process source that imposes the requirement. If the process source is inferred from repository convention, say so and keep severity below blocking unless the production risk is clear.

## Runtime Integration Checks

- Connect process rules to executable gates: tests, validators, approval endpoints, Docker checks, CI commands, docs, and release/runbook updates.
- Verify new operator-facing capabilities update setup/configuration, status,
  inspection, approval/finalization, or recovery documentation when those
  surfaces exist.
- Durable knowledge stores should receive only approved decisions with citations;
  draft, rejected, or speculative notes must not be persisted.
- Process findings should identify whether the missing artifact blocks local development, production deploy, incident recovery, or auditability.

## Review Output Contract

Return process findings as missing operational evidence, not vague documentation requests. Include the source rule, changed behavior, missing artifact, responsible lifecycle stage, and concrete command or document section that should be added.

## False Positive Checks

- Do not demand documentation for internal refactors with no observable behavior or operational change.
- Do not enforce stale process documents over newer explicit project rules; surface the conflict instead.
- Do not require heavyweight release process for isolated tests, formatting, or local-only tooling.
- Do not duplicate findings already covered by security, API, data migration, or reliability skills unless the process obligation is separate.

## Review Questions

- Does the change require tests, docs, migration notes, or rollout steps?
- Does it alter behavior without updating process-owned artifacts?
- Does it require HIL approval before publishing or finalizing?
- Does it need operational observability or rollback guidance?
- Does it violate the team's accepted delivery sequence?

## Conditional Process Surface Map

Use these rows only when the selected corpus or diff contains the matching
surface. They are examples of process obligations, not assumptions about the
target repository:

- Webhook or external event entry: secret/signature validation, queueing, duplicate delivery handling, and durable state creation.
- Automated analysis workflow: source enrichment, input normalization, context selection, model/tool execution, validation, report rendering, and draft output.
- Human iteration workflow: chat, follow-up questions, suppression, approval, final publishing, and audit trail.
- Knowledge loop: approved decisions may become durable memory; selected context may be compressed only if citations survive.
- Deployment loop: runtime services must have config validation, health/readiness checks, network reachability, and operator-visible failures.

## Process Evidence Matrix

When a process rule appears relevant, map it before filing a finding:

| Change Type | Expected Companion Evidence |
| --- | --- |
| New webhook/API behavior | webhook tests, auth/secret rejection tests, docs/config update |
| New model/prompt behavior | deterministic parser/validator tests, prompt contract update, fallback behavior |
| New memory behavior | approval boundary, persistence test, privacy/redaction check |
| New runtime dependency | config validation, health/readiness check, docs update |
| New operator command | command parsing test, auth behavior, user-facing error path |

## Test Expectations

- Process findings should reference an existing rule, docs section, or repeatable repository convention.
- Missing documentation findings should name the exact doc or command help that should change.
- Release/readiness findings should include the production action that would fail without the missing process step.

## Finding Template

```md
### [severity] Process knowledge gap

- Process source: `<DoD/runbook/release gate/review policy>`
- Changed behavior: `<code or artifact affected>`
- Missing step: `<test/doc/approval/rollout/rollback/observability>`
- Evidence: `<source citation plus changed lines>`
- Production impact: `<risk created by skipping the step>`
- Suggested fix: `<specific artifact or gate to add/update>`
```
