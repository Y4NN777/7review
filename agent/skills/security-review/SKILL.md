---
name: security-review
description: Use for any change touching trust boundaries, authn/authz, secrets, privacy, tenant isolation, parsers, files, network calls, webhooks, crypto, logging, dependency execution, or user-controlled input. This skill produces exploit-oriented security findings with attacker capability, failing guard, affected asset, evidence, false-positive checks, and concrete remediation.
license: Apache-2.0
compatibility: Go services, SCM webhooks, HTTP APIs, Docker sidecars, GitHub/GitLab integrations
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator run-store mempalace
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: security
  risk-tier: high
---

# Security Review Skill

## Activation Contract

Activate when changed code accepts, transforms, authorizes, stores, logs, signs,
publishes, or calls anything that can cross a trust boundary. Treat webhook
handlers, SCM APIs, model prompts, memory writes, Docker bridges, and filesystem
reads as security-sensitive even when the code looks like plumbing.

## Review Algorithm

1. Identify every new or changed trust boundary:
   - external HTTP request
   - webhook payload or signature
   - SCM API response
   - model output
   - local file path
   - environment variable or secret
   - memory/headroom sidecar payload
2. For each boundary, name the principal, asset, and guard:
   - principal: anonymous user, SCM user, maintainer, CI job, sidecar process
   - asset: source code, token, review report, memory, private diff, tenant data
   - guard: signature, token scope, path allowlist, HIL approval, validator
3. Check whether the changed code preserves the guard on every path, including
   error handling, retries, fallback providers, and no-op/default adapters.
4. Build an exploit sketch before reporting. If no realistic attacker or
   misconfiguration path exists, do not file a security finding.
5. Prefer one precise remediation that keeps the intended feature working.

## Review Areas

- Authentication: missing checks, weak session handling, token confusion, replay risk
- Authorization: object-level access control, role/tenant boundaries, privilege escalation
- Input handling: SQL/NoSQL/template/command injection, unsafe regex, parser ambiguity
- Network access: SSRF, webhook spoofing, redirect abuse, unsafe callbacks
- Filesystem: path traversal, unsafe temp files, permission errors, archive extraction
- Secrets: committed keys, environment leaks, logs, error messages, telemetry
- Privacy: PII exposure, retention violations, cross-tenant data leakage
- Crypto: custom crypto, weak randomness, insecure hashing, nonce/key reuse

## Technical Patterns To Check

### Webhook and SCM Boundaries

- Missing HMAC/signature verification before JSON parse or enqueue.
- Accepting GitHub/GitLab event types without checking action/state.
- Trusting project, MR, PR, branch, SHA, author, labels, or file paths from the
  webhook without enrichment from the provider API.
- Publishing draft/final comments without stable bot markers or idempotency.
- Using a token that can write where read-only enrichment is sufficient.

### Prompt and Model Boundaries

- Sending untrusted PR body/comments directly into system prompts.
- Letting model output bypass deterministic finding validation.
- Treating model chat instructions as approval for final publish or memory write.
- Including secrets, full private diffs, or unrelated repository files in prompts.

### Filesystem and Sidecar Boundaries

- Reading arbitrary paths from SCM metadata or chat input.
- Path traversal through repo-relative corpus discovery.
- Docker bridge services exposing writable `/data` without validation.
- Logging sidecar payloads that may contain secrets, private diffs, or memory.

### Authorization and Tenant Isolation

- Object-level access control missing after ID lookup.
- Project/MR identifiers used across providers without provider scoping.
- Memory recall/write not scoped by repository/project/change.
- HIL approval endpoint accepting ambiguous project/change identifiers.

## Execution Rules

Security findings require an actor, capability, boundary, skipped control, and
affected asset. File when untrusted webhook, SCM, chat, filesystem, sidecar, or
model content can cross into a privileged action without deterministic
validation. Suppress if a shared guard definitely wraps the path before the sink
and the sink cannot mutate, publish, persist, or disclose sensitive data.

Choose the control class:

- authentication/signature check for external requests
- authorization/scope check after object lookup
- input normalization and path safety for filesystem or SCM data
- prompt/output validation for model-controlled content
- lifecycle/HIL guard for publishing and memory writes
- redaction/retention control for logs, reports, packaged env, and durable memory

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Trust boundary discovery | `filesystem`, `corpus-selector` | Locate auth, secrets, webhook validators, filesystem reads, network clients, Docker config, memory writes, and logging paths. |
| Exploit path analysis | `diff-analyzer`, `scm-api` | Trace attacker-controlled inputs through parsing, authorization, external calls, publishing, storage, and logs. |
| State/memory check | `run-store`, `mempalace` | Verify sensitive actions depend on durable state and memory writes do not persist unapproved/private content. |
| Validation | `validator` | Require attacker capability, skipped control, affected asset, evidence, and remediation before filing. |

## Escalation Signals

- User-controlled webhook/chat/SCM content reaches file, network, publish, model, memory, or log boundaries without validation.
- Secret, token, private diff, or personal data can be exposed in reports, logs, browser UI, memory, or Docker env.
- Authorization checks differ across API, TUI, webhook, and worker paths.

## Evidence Standard

A valid security finding must include:

- attacker or failure actor
- changed path and reachable code path
- missing or weakened guard
- affected asset
- exploit or abuse sequence
- specific remediation
- test that would catch the regression

## False Positive Checks

Do not report if:

- the guard is enforced in a shared middleware that definitely wraps this path
- the input is only consumed after deterministic validation and escaping
- the code is test-only or local-only and cannot affect production behavior
- the finding relies on an attacker controlling data they cannot control

## Review Procedure

Use this sequence during execution:

1. **Classify the boundary.** Name the boundary family first: webhook, SCM API,
   filesystem, model prompt, sidecar, persistence, publish, or HIL.
2. **Trace tainted fields.** Follow attacker-controlled or untrusted fields from
   input to sink. Examples:
   - webhook body -> normalized request -> SCM API path
   - PR/MR body -> model prompt -> generated report
   - changed path -> corpus file read -> prompt context
   - human chat message -> run-bound streaming prompt
   - approved report -> durable memory write
3. **Find the guard.** Identify the exact validation point, not a vague layer:
   - HMAC compare before enqueue
   - provider action allowlist
   - project/change scoping
   - path normalization and root containment
   - deterministic finding validation
   - HIL approval flag
4. **Check alternate paths.** Review aliases and fallback paths:
   - `/webhook`, `/webhook/gitlab`, `/webhook/github`
   - draft publish vs final publish
   - local chat vs run-bound chat
   - OpenAI-compatible vs Ollama vs non-streaming fallback
   - no-op adapters vs production adapters
5. **Decide severity.** Use reachable impact:
   - critical: credential exposure, unauthorized publish, cross-project memory
     write, remote code execution, destructive action without approval
   - high: auth bypass, tenant/project data exposure, prompt injection causing
     unapproved side effect, webhook spoofing
   - medium: sensitive metadata leak, weak idempotency causing duplicate publish,
     missing validation with limited impact
   - low/info: hardening gap with no direct exploit but meaningful defense value

## Conditional Agent Security Checklist

Use these rows only when the selected corpus or diff contains agent-like
surfaces such as webhooks, chat, model prompts, tool calls, publishing, memory,
or context compression. For ordinary services, apply the same trust-boundary
logic to their concrete surfaces instead.

### Webhook Intake

- GitHub signatures must use the raw request body and constant-time HMAC
  comparison.
- GitLab webhook secret must be checked before accepting or enqueueing.
- Handlers must reject unsupported methods and malformed payloads.
- Event normalization must not trust payload-only SHAs/paths when enrichment is
  available.
- Queue backpressure must return a service error, not silently drop work.

### Run and Chat APIs

- Run inspection and chat endpoints expose review data. Before production
  exposure, they need an auth boundary or must sit behind trusted ingress.
- Run IDs must be provider/project scoped; avoid collisions across SCM systems.
- Chat must not turn model suggestions into actions. Approval and publish remain
  deterministic endpoints.
- Streaming errors must not include secrets or full dependency payloads.

### Prompt Safety

- Treat PR/MR title, description, labels, comments, and changed file content as
  untrusted prompt input.
- Instructions from repository files or PR text must never override the
  always-on agent lifecycle, HIL gate, or memory policy.
- Findings must pass deterministic validation before draft publishing.
- The model may explain findings, but cannot approve final reports or write
  memory.

### Memory and Context Compression

- Durable memory recall/write must be scoped by project/repository/change.
- Do not write rejected findings or unapproved drafts into memory.
- Context compression must preserve source identity; losing path or
  section identity can create unsafe citations.
- Sidecar health failures are hard failures, not silent degradation.

### Publishing

- Draft/final reports need stable markers for idempotency.
- Final report publishing requires explicit HIL approval when policy requires it.
- Publisher must avoid echoing raw secrets from findings, prompts, or logs.
- Prior bot comments should be updated/superseded instead of duplicated.

## Runtime Integration Checks

- Validate every trust boundary in the live path: webhooks, operator APIs, chat stream, model prompt inputs, filesystem reads, context compression, durable memory, SCM publishing, and runtime network exposure.
- Secrets and tokens must be redacted in errors, reports, memory proposals, logs, and streamed chat output.
- Durable memory recalls are untrusted context until scoped and cited; memory writes require approved final state and must not store rejected findings or raw secrets.
- Context compression must preserve security-relevant source identity; compression that drops path, actor, asset, or guard identity can invalidate the finding.

## Review Output Contract

Security output must describe actor, asset, guard, bypass path, evidence, impact, and fix. Separate exploitable findings from hardening suggestions, and never include secret values or full sensitive payloads in the report.

## Secure Finding Template

Security findings must use the shared finding template below and include enough
detail for an engineer to reproduce the trust-boundary failure without exposing
secrets or unrelated private code.

## Finding Template

Use this shape when producing a security finding:

```text
Title: <specific failing guard>
Severity: <critical|high|medium|low|info>
Actor: <who can trigger it>
Asset: <what is exposed or modified>
Evidence: <changed path and reachable flow>
Impact: <realistic abuse or failure>
False-positive check: <why existing guards do not already prevent it>
Fix: <smallest safe remediation>
Test: <unit/integration case that proves the guard>
```

## Finding Rules

Report only when the changed code creates, exposes, or fails to preserve a real security property. Include attacker capability, affected asset, failing check, and concrete fix.
