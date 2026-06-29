---
name: data-migration-review
description: Use for database migrations, schema changes, data model updates, persistence logic, query behavior, cache invalidation, backfills, retention changes, and durable memory writes. This skill reviews rolling deploy safety, reversibility, locking, data integrity, tenant isolation, migration ordering, and operational recovery.
license: Apache-2.0
compatibility: SQL/NoSQL stores, ORMs, migrations, caches, queues, vector stores, durable memory systems
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator mempalace
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: data-migration
  risk-tier: high
---

# Data Migration Review Skill

## Activation Contract

Activate when a change affects durable state or the interpretation of durable
state. This includes schema migrations, model fields, indexes, queries,
serializers, cache keys, backfills, retention policies, vector/memory writes, and
data import/export scripts.

## Review Algorithm

1. Identify schema, query, migration, cache, or persistence behavior touched by the change.
2. Check forward and backward compatibility across rolling deploys.
3. Look for data loss, lock amplification, slow queries, and unsafe backfills.
4. Confirm application code and migration order are compatible.
5. Require validation, rollback, or operational guardrails for risky data changes.
6. Check whether the change is safe with production-sized data and repeated
   execution.

## Review Areas

- Adding non-null columns, defaults, indexes, constraints, and foreign keys
- Renames and destructive changes
- Query plan risk, missing indexes, unbounded scans
- Transaction boundaries and isolation assumptions
- Cache invalidation and stale reads
- Backfill batching, resume behavior, and observability
- Multi-tenant and privacy isolation in persistence logic

## Technical Patterns To Check

### Rolling Deploy Compatibility

- New code reads a column/field before the migration creates it.
- Old code cannot tolerate new values during a rolling deploy.
- Migration and application deploy order is not documented or enforced.
- JSON/protobuf persisted shape changes without backward readers.
- Required field added without default, nullable transition, or dual-write plan.

### Destructive or Irreversible Changes

- Dropping columns, collections, indexes, or keys before all readers/writers stop
  using them.
- Renaming fields without copy/dual-read/dual-write period.
- Data deletion without backup, retention rule, or HIL/ops approval where needed.
- Down migration cannot restore lost data but is presented as reversible.

### Locking and Query Plan Risk

- Table rewrite from non-null default or type conversion on large tables.
- Index creation without concurrent/online mode where the database requires it.
- Foreign key or uniqueness validation that scans production data synchronously.
- Query adds unbounded sort, join, regex, or filter without supporting index.
- Pagination query becomes unstable under concurrent inserts.

### Backfill and Batch Safety

- Backfill has no batch size, checkpoint, resume token, dry-run, or rate limit.
- Backfill runs in app startup path or request path.
- Failure halfway leaves ambiguous state or cannot be retried idempotently.
- Progress and error metrics are missing for long-running data jobs.

### Cache and Derived Data

- Cache key changes without invalidation or versioning.
- Stale cached authorization/privacy data can leak access.
- Derived projections or search/vector indexes are not rebuilt after schema change.
- Event replay creates duplicate rows because idempotency keys are missing.

### Memory and Vector Stores

- Memory writes are not scoped by project/repository/change.
- Rejected findings or unapproved drafts can be persisted.
- Vector text is written without source, timestamp, or approval metadata.
- Embeddings are updated without handling stale vectors.

## Execution Rules

Data migration findings require a concrete old/new data shape and an unsafe
transition between deployed versions. File when the diff assumes atomic deploys,
breaks rollback, loses data, writes durable memory too early, or makes readers
unable to tolerate both shapes. Suppress when migration is reversible, bounded,
observable, and code supports mixed versions.

Pick the migration remedy:

- expand/contract sequencing for schema and stored JSON changes
- compatibility reader/writer for rolling deploys
- backfill controls for large or durable stores
- privacy/redaction handling for memory and report persistence
- rollback notes and tests for irreversible transitions

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Persistence map | `filesystem`, `corpus-selector` | Locate schemas, migrations, model structs, persistence adapters, cache keys, run-store formats, and memory contracts. |
| Change impact | `diff-analyzer`, `scm-api` | Compare schema/data-shape changes against read/write paths, rollout order, backfill behavior, and rollback assumptions. |
| Memory boundary | `mempalace` | When durable agent memory is involved, verify that writes happen after approved final state and preserve citations. |
| Validation | `validator` | Require evidence for data loss, lock risk, incompatibility, or migration ordering before filing. |

## Escalation Signals

- A data shape changes without backward/forward compatibility across rolling deploys.
- Migration, cache, run-store, or memory writes cannot be retried idempotently.
- Draft or rejected review information can become durable memory.

## Evidence Standard

A data finding must include:

- affected store/table/collection/cache/index
- changed data shape or query behavior
- rolling deploy or operational failure mode
- realistic production data-volume concern
- rollback or recovery impact
- smallest safe migration sequence

## Runtime Integration Checks

- Treat run-store JSON, SCM provider metadata, durable memories, compressed context packs, caches, and database schemas as durable data surfaces.
- Verify writes are scoped by provider, repository, change ID, and run ID so GitHub/GitLab or multiple projects cannot collide.
- Check that final-approved data and draft/rejected data use separate lifecycle paths; draft findings must not become durable memory.
- For rolling deploys, confirm old readers tolerate new fields and new readers tolerate old records.

## Review Output Contract

Report data risks as a sequence problem: what shape exists before deploy, what the new code expects, which operation crosses the boundary, and what expand/backfill/dual-read/contract step is missing. Include the rollback or replay consequence.

For `confirmed` findings, include structured citations: `source`,
`heading_or_key`, `rule`, and `violation`. The `rule` must match selected data,
contract, migration, or operational source evidence. TTL, pruning, and
performance concerns without a cited requirement, measured risk, missing
required index, or realistic volume evidence must be notes/questions, not
confirmed findings.

## False Positive Checks

Do not report if:

- the store is ephemeral and can be safely rebuilt
- the change is behind a proven migration sequence with tests
- the dataset is bounded and the code enforces that bound
- the migration is intentionally one-way and the operational approval is explicit

## Safe Migration Playbook

Prefer this sequence for risky schema changes:

1. Expand: add nullable fields, new tables, or new indexes without removing old
   readers.
2. Dual-write or backfill with checkpoints and observability.
3. Dual-read with fallback and validation.
4. Flip read path behind a controlled rollout.
5. Contract: remove old fields only after all readers and backfills are complete.

## Finding Template

```text
Title: <data safety risk>
Store: <table/collection/cache/index>
Failure mode: <what breaks under rolling deploy or production volume>
Evidence: <changed file and data path>
Impact: <data loss/corruption/leak/outage>
Fix: <safe migration sequence or query/index change>
Test/operation: <migration test, rollback check, or production guard>
```

## Finding Rules

Prioritize issues that can break deploys, corrupt data, leak tenant data, or create production incidents under realistic data volume.
