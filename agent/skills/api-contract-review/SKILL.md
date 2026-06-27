---
name: api-contract-review
description: Use for changes to REST, GraphQL, RPC, protobuf, OpenAPI, AsyncAPI, webhooks, SDK-facing structs, HTTP status semantics, pagination, idempotency, retry behavior, request/response schemas, and event payloads. This skill detects contract drift, backward-incompatible behavior, unsafe defaults, and missing contract tests.
license: Apache-2.0
compatibility: HTTP APIs, GitHub/GitLab webhooks, REST clients, JSON schemas, Go structs
allowed-tools: scm-api filesystem diff-analyzer corpus-selector validator
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: api-contract
  risk-tier: high
---

# API Contract Review Skill

## Activation Contract

Activate when changed code alters a boundary consumed by another process,
service, browser, CLI, webhook sender, SDK, or stored event. Internal structs are
contracts when they are serialized, persisted, exposed through tools, or used by
sidecars.

## Review Algorithm

1. Identify public or internal API surface touched by changed files.
2. Compare behavior against selected OpenAPI, AsyncAPI, protobuf, schema, SRS, or client contracts.
3. Check backward compatibility for consumers.
4. Validate error, retry, and idempotency semantics.
5. Require docs/spec/test updates when behavior changes.
6. Verify generated clients, examples, and `.env.example` style config samples
   match the new behavior.

## Review Areas

- Request and response schema compatibility
- Required vs optional fields
- Enum expansion and unknown-value handling
- Pagination, sorting, filtering, and cursor stability
- Status codes, error body shape, and retryability
- Webhook signature verification and event replay handling
- Idempotency keys for mutation endpoints
- Timeout, cancellation, and partial-failure behavior

## Technical Patterns To Check

### Schema Compatibility

- Required field added to request without default or version gate.
- Response field removed, renamed, changed type, or changed nullability.
- Enum narrowed or switch logic rejects unknown future values.
- Numeric precision or time format changed.
- JSON tags changed on persisted/API structs.

### HTTP Semantics

- New status code not documented or not handled by clients.
- Returning `200` for accepted async work that can still fail.
- Returning raw dependency errors or provider messages as public API responses.
- Error body shape drift: `message` vs `error`, missing code, missing details.

### Pagination and Filtering

- Cursor not stable under inserts/deletes.
- Sort default changed without migration note.
- Page size limit removed or no maximum enforced.
- Filters apply after pagination rather than before.

### Webhooks and Events

- Signature verification order changed.
- Event replay/idempotency missing.
- Missing provider/action filtering.
- Payload normalization drops IDs required for idempotent publishing.

### Tool/Agent API Contracts

- Tool schemas too vague for model-safe use.
- Chat command or endpoint takes free-form IDs without provider scoping.
- Streaming response missing terminal `done` event or error event shape.

## Execution Rules

Treat the declared contract as stronger than implementation convenience. File a
finding when the diff changes observable behavior and at least one consumer can
receive a shape, status, timing, retry, or idempotency result that the selected
contract does not allow. Suppress the finding when the contract and all generated
or documented consumers move together, or when the break is explicitly versioned
and opt-in.

Choose the remediation class before writing the finding:

- code compatibility shim when existing consumers can break
- contract/spec update when implementation intentionally changes behavior
- generated client or example update when downstream artifacts are stale
- regression/contract test when the rule exists but is not executable

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Contract lookup | `filesystem`, `corpus-selector` | Locate OpenAPI, AsyncAPI, protobuf, SDK structs, webhook schemas, generated clients, and contract tests before judging compatibility. |
| Behavior comparison | `scm-api`, `diff-analyzer` | Compare changed files, removed fields, renamed paths, status codes, pagination, retry/idempotency semantics, and generated artifacts. |
| Deterministic gate | `validator` | Reject findings that do not identify the contract, consumer impact, evidence path, and compatible remediation. |

## Escalation Signals

- A public request/response field, event field, enum value, status code, or endpoint is removed or renamed without compatibility handling.
- Runtime behavior changes but generated schema, SDK, docs, or contract tests do not change.
- The implementation accepts or returns shapes that conflict with the declared schema.

## Evidence Standard

An API finding should cite:

- old and new contract behavior
- consumer that can break
- request/response/event field involved
- migration or compatibility path
- required test or spec update

## Runtime Integration Checks

- Confirm the normalized review request, SCM enrichment payload, chat endpoint, and TUI commands use the same provider-scoped identifiers and schema names.
- When contracts are reduced through context compression, require path, section, version, and endpoint identifiers to survive compression.
- If durable memory recalls a historical contract decision, treat it as advisory until the current repository contract or API spec confirms it.
- Verify publisher output distinguishes implementation findings from contract/documentation update requests.

## Review Output Contract

Return only contract issues that name the public surface, consumer, changed field or behavior, and compatibility path. Each issue must say whether the fix belongs in code, schema/docs, generated clients, tests, or migration guidance. Do not emit generic "update docs" advice without the exact contract artifact.

## False Positive Checks

Do not report if:

- the endpoint is explicitly versioned and consumers opt into the break
- a compatibility shim preserves old behavior
- the changed field is internal-only and never serialized/persisted
- tests and contract docs are updated to match an intentional break

## Finding Template

```text
Title: <contract drift or incompatible API behavior>
Severity: <critical|high|medium|low|info>
Surface: <REST/GraphQL/RPC/webhook/tool/schema>
Consumer: <client, service, sidecar, browser, SCM provider, memory bridge>
Old behavior: <documented or observed contract>
New behavior: <changed behavior>
Evidence: <changed path plus contract source>
Compatibility impact: <who breaks and how>
Fix: <compat shim, versioning, spec update, or test>
```

## Finding Rules

Flag changes that silently break clients, drift from the contract, or change behavior without updating the source contract and tests.
