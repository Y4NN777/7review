---
name: reliability-review
description: Use for changes touching goroutines, workers, queues, retries, timeouts, context cancellation, HTTP clients, external APIs, Docker sidecars, streaming, memory stores, file handles, locks, schedulers, idempotency, or observability. This skill finds production failure modes such as leaks, hangs, duplicate work, lost work, retry storms, and silent partial failure.
license: Apache-2.0
compatibility: Go services with HTTP, worker queues, streaming, SCM APIs, Docker sidecars
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator run-store headroom mempalace
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: reliability
  risk-tier: high
---

# Reliability Review Skill

## Activation Contract

Activate when a diff changes execution lifecycle: background work, external
calls, streaming, retries, concurrency, queueing, readiness, shutdown, storage,
or any code that must keep working under partial failure.

## Review Algorithm

1. Identify new failure modes introduced by changed control flow or integrations.
2. Check resource lifecycle: cancellation, cleanup, closing, and bounded concurrency.
3. Evaluate retry, timeout, and idempotency behavior.
4. Confirm errors are handled with enough context and do not hide failed state.
5. Trace what happens when each dependency is slow, down, returns malformed data,
   times out, or partially succeeds.
6. Prefer findings tied to realistic production failures, not theoretical style.

## Review Areas

- Goroutine leaks, race-prone shared state, deadlocks, unbounded fan-out
- Missing context propagation, timeout, or cancellation handling
- Retry storms, missing backoff, non-idempotent retries
- Partial writes, duplicate jobs, lost acknowledgements
- Resource leaks: files, response bodies, database rows, timers
- Missing metrics/logs for critical failure paths
- Panic risks and unsafe nil/error handling

## Technical Patterns To Check

### Context and Timeout Discipline

- External HTTP calls without `context.Context` or client timeout.
- Reusing a background context where request cancellation should propagate.
- Streaming loops that ignore cancellation after the client disconnects.
- Health/readiness checks that can hang longer than orchestrator or load balancer
  budgets.

### Worker and Queue Safety

- Unbounded goroutines per webhook, file, finding, or chat message.
- Worker queue overflow that silently drops work instead of returning backpressure.
- Panic in worker path without recovery and contextual logging.
- Duplicate job handling missing for repeated webhooks or retries.

### Retry and Idempotency

- Retrying non-idempotent publish/write operations without idempotency markers.
- Sidecar writes that can partially commit and then be retried as a duplicate.
- Missing backoff or jitter around SCM, context compression, durable memory, sidecar, and model calls.
- Treating 4xx and 5xx errors the same.

### Resource Cleanup

- HTTP response body not closed on all paths.
- Scanner or decoder loops without max-size limits for untrusted streams.
- Temp files and directories not cleaned up.
- Locks held across I/O, model calls, or network calls.

### Observability

- Errors swallowed or returned without provider, run ID, project, change ID, path,
  or dependency name.
- Readiness reports "ok" while required dependencies are degraded.
- Missing log when a fallback provider is used.

## Execution Rules

Reliability findings must identify a failure mode and the boundary that handles
it incorrectly. File when the changed code can hang, leak work, retry unsafely,
hide dependency failure, corrupt run state, or leave operators without enough
signal to recover. Suppress when the path is best-effort and the failure is
explicitly isolated from production state.

Choose the repair type:

- timeout/context propagation for unbounded waits
- idempotency/backoff for retryable side effects
- readiness or health signal for required dependencies
- persistence/state repair for crash or restart safety
- structured error/log context for operator recovery

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Runtime path discovery | `filesystem`, `corpus-selector` | Locate goroutines, workers, queueing, HTTP clients, retries, timeouts, streaming loops, stores, locks, and sidecar calls. |
| Failure-mode analysis | `diff-analyzer`, `scm-api` | Trace cancellation, retry, idempotency, ordering, duplicate delivery, partial failure, and shutdown behavior through changed paths. |
| State and sidecar checks | `run-store`, `headroom`, `mempalace` | Verify durable state, context reduction, and memory calls have timeout, retry, and error semantics. |
| Validation | `validator` | Require an executable failure path, lost/duplicated work scenario, leak, hang, or observability gap. |

## Escalation Signals

- Background work uses unbounded context, unbounded queue, unbounded retry, or no idempotency marker.
- External calls can hang or silently drop partial results.
- Shutdown, cancellation, redelivery, or stream disconnect behavior is undefined.

## Evidence Standard

A reliability finding must describe:

- failure trigger
- path through changed code
- resulting user/operator impact
- why existing timeouts/retries/idempotency do not contain it
- concrete guard or test to add

## Runtime Integration Checks

- Check every external call path for bounded context, client timeout, response-size limit, body close, and dependency-specific error context.
- Worker queues must handle duplicate delivery, overflow, panic, shutdown, and job timeout without losing observable state.
- Streaming chat must handle client disconnects, partial chunks, scanner limits, terminal events, and model/provider failures.
- Required sidecar or external dependency calls must fail explicitly in production readiness and review execution, not degrade silently.

## Review Output Contract

Reliability output must be an executable failure scenario: trigger, code path, missing containment, user/operator impact, and deterministic test. Prefer one concrete incident path over broad advice about adding retries or logs.

For `confirmed` findings, include structured citations: `source`,
`heading_or_key`, `rule`, and `violation`. The `rule` must match selected
reliability, contract, runbook, or architecture evidence. The `violation` must
describe a reachable failure path from the changed line; otherwise use
`strength=likely`, `speculative`, or `finding_type=note`.

## False Positive Checks

Do not report if:

- the operation is intentionally best-effort and caller observes degradation
- a higher-level wrapper provides the missing timeout/retry/cleanup
- the resource is memory-only and bounded by existing limits
- a duplicate operation is proven idempotent by stable keys or markers

## Finding Template

```text
Title: <specific production failure mode>
Severity: <critical|high|medium|low|info>
Trigger: <slow dependency, retry, cancellation, queue overflow, shutdown, etc>
Evidence: <changed path and reachable code path>
Impact: <hang, leak, duplicate work, lost work, hidden failure, operator blind spot>
Existing guard check: <why current timeout/retry/idempotency does not contain it>
Fix: <bounded lifecycle, backoff, cleanup, observability, or idempotency guard>
Test: <fake dependency or unit/integration case that proves the behavior>
```

## Finding Rules

Report when the change can hang, leak, duplicate work, lose work, hide errors, or fail under load in a way tests or logs would not catch.
