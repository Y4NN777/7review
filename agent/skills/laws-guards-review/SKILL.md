---
name: laws-guards-review
description: Use for protected behavior governed by laws, guards, invariants, prohibitions, safety boundaries, authorization constraints, privacy rules, operation contracts, approval gates, destructive actions, publishing, and memory writes. Finds bypasses and alternate paths that violate explicit constraints.
license: Apache-2.0
compatibility: Repositories with invariants, threat controls, operation contracts, approval policies, or safety rules
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator run-store
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: laws-guards
  risk-tier: critical
---

# Laws Guards Review Skill

## Purpose

Use this skill when changes touch protected behavior: auth, identity, privacy, moderation, billing, storage, sync, deployment, destructive actions, or external publishing.

## Activation Contract

Activate when the consequence of a bug is not just wrong output but a forbidden
state or forbidden action. This includes authorization, privacy, destructive
mutation, publish/finalize actions, durable memory writes, billing, moderation,
and safety controls.

## Guard Families

- `LAW-*`: conflict-resolution or system laws
- `INV-*`: invariants that must always hold
- `PRO-*`: prohibited actions or states
- `GAR-*`: guarantees to users or systems
- `CTRL-*`: threat-model controls
- `OPC-*`: operation contracts
- `HIL-*`: human approval gates
- `MEM-*`: memory retention/write rules

## Review Algorithm

1. Identify the guard family and authoritative source.
2. State the protected asset or forbidden state.
3. Trace every path that can reach the guarded action.
4. Check fallback, retry, chat, API, CLI, and no-op/default paths for bypasses.
5. Confirm tests prove the guard fails closed.
6. If the guard is missing, suggest the smallest constraint that blocks the
   bypass without over-constraining valid behavior.

## Review Questions

- Can the change bypass a guard through an alternate path?
- Does it weaken validation, authorization, or approval?
- Does it create a state that violates an invariant?
- Does it publish, delete, mutate, or persist data without the required gate?
- Does it make an operation non-idempotent or unsafe under retries?
- Can model output, chat input, or webhook payload trigger an action that should
  require deterministic validation or HIL?
- Are rejected or unapproved findings excluded from final publish and memory?

## Technical Patterns To Check

### Guard Placement

- Guard happens after side effects instead of before.
- Guard exists in one route but not alias routes or CLI path.
- Guard is enforced by UI only, not server/pipeline.
- Guard checks user intent but not object/project scope.

### Fail-Closed Behavior

- Missing dependency treated as success.
- Unknown enum/action defaults to allowed.
- Error path publishes, writes memory, or marks approval complete.
- No-op adapter masks required production enforcement.

### Alternate Path Bypass

- Draft path can publish final content.
- Chat/model response can be interpreted as approval.
- Retry path skips validation.
- Direct HTTP endpoint bypasses queue/policy/validator.
- GitHub and GitLab paths enforce different gates.

### Idempotency and Irreversibility

- Destructive or external action has no stable idempotency key.
- Memory write cannot distinguish accepted vs rejected findings.
- Duplicate publish creates conflicting review state.
- Rollback cannot restore protected state.

## Execution Rules

Guard findings are filed as bypasses. A valid finding needs a protected action,
a required condition, and a changed or newly reachable path that can perform the
action without the condition. Treat fail-open defaults, missing dependency
success, UI-only enforcement, and provider-specific divergence as high risk.
Suppress when a stronger guard wraps every path before side effects happen.

Select the enforcement point before writing:

- route/auth guard for external HTTP or CLI entrypoints
- validator gate for model output, findings, and reports
- run-state guard for approval, final publish, and memory writes
- adapter guard for SCM, context compression, durable memory, or filesystem side effects

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Guard source lookup | `filesystem`, `corpus-selector` | Locate explicit laws, guards, policy docs, invariants, prohibited actions, auth rules, HIL rules, and operation contracts. |
| Bypass analysis | `diff-analyzer`, `scm-api` | Trace changed paths for alternate entry points, retries, background jobs, chat commands, webhooks, and publisher/memory routes. |
| State verification | `run-store` | When guards depend on run state, verify the durable state transition rather than trusting request text. |
| Validation | `validator` | Require a concrete bypass path and affected asset before filing. |

## Escalation Signals

- One code path enforces a guard but another entry point reaches the same mutation without it.
- A destructive, publishing, approval, privacy, or memory action can run from ambiguous state.
- A guard is documented but not bound to deterministic code.

## Evidence Standard

A laws/guards finding must include:

- guard ID/family or explicit rule source
- guarded action or forbidden state
- bypass path through changed code
- impact of entering the forbidden state
- fail-closed remediation
- regression test that proves the guard

## Runtime Integration Checks

- Trace guards across all entrypoints: webhook, worker, chat stream, approve, final publish, setup, status, Docker readiness, and sidecar bridges.
- Check alternate paths, not only the happy path; a guard in the UI is insufficient if HTTP or worker code can bypass it.
- Required production dependencies such as context compression or durable memory services must fail closed when unavailable.
- Guard evidence must survive context reduction and publishing so the final report can explain why the gate exists.

## Review Output Contract

Frame each guard issue as a bypass: protected action, required condition, alternate path, missing enforcement point, and impact. Include the exact validator, config check, route auth, state check, or adapter guard that should contain it.

## False Positive Checks

Do not report if:

- the action is read-only and cannot mutate/publish/persist protected state
- a stronger guard wraps all reachable paths
- the rule is advisory, not a hard invariant/prohibition
- the change removes the guarded action entirely

## Finding Template

```text
Title: <guard bypass or invariant violation>
Guard: <LAW/INV/PRO/GAR/CTRL/OPC/HIL/MEM>
Protected action/state: <what must not happen>
Bypass path: <changed code path>
Impact: <forbidden result>
Fix: <fail-closed guard or narrower permission>
Test: <case that proves the bypass is blocked>
```

## Finding Rule

A laws/guards finding must identify the guard, the bypass or violation, and the smallest corrective constraint.
