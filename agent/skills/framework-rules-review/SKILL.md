---
name: framework-rules-review
description: Use to enforce repository-specific engineering framework rules, architectural boundaries, coding conventions, language idioms, lifecycle hooks, package ownership, local helpers, deterministic gates, and documented guardrails. This skill resolves local rules from AGENTS.md, CLAUDE.md, rules directories, nearby code patterns, and framework docs without turning style preference into findings.
license: Apache-2.0
compatibility: Repositories with local agent instructions, framework docs, package boundaries, coding standards, or established helper APIs
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: framework-rules
  risk-tier: medium
---

# Framework Rules Review Skill

## Purpose

Use this skill to evaluate whether changed code follows the project's local
engineering framework. This is not a generic style checker. It is for local
rules that protect maintainability, correctness, security, reliability, and
reviewability.

## Activation Contract

Activate for non-trivial code changes where correctness depends on local
architecture, framework lifecycle, package ownership, conventions, helper APIs,
or repository-specific rules.

## Rule Sources

- `AGENTS.md`, `CLAUDE.md`, `agent/instructions.md`
- `.codex/**`, `.claude/**`
- `rules/**`, `RULES/**`, `docs/rules/**`
- architecture contracts, ADRs, component maps
- generated project wiki/context pack review maps
- language/framework conventions in project docs
- existing nearby code patterns
- established package ownership boundaries

## Rule Resolution Order

When sources disagree, resolve in this order:

1. Explicit repository instructions in `AGENTS.md`, `CLAUDE.md`, or
   `agent/instructions.md`.
2. Domain-specific rules under `rules/**`, `docs/rules/**`, `.claude/**`, or
   `.codex/**`.
3. Architecture contracts, ADRs, and component ownership docs.
4. Existing nearby implementation patterns in the same package/module.
5. General language/framework idioms.

If the rule source is ambiguous, ask for confirmation or file a low-severity
traceability/process note instead of a hard finding.

## Review Algorithm

1. Identify the touched framework surface:
   - entrypoint/server wiring
   - handler/controller
   - adapter/tool/provider
   - pipeline/workflow state
   - model orchestration
   - UI/TUI
   - config/deployment
   - tests
2. Locate the authoritative local rule or nearest established pattern.
3. Compare the change against ownership, lifecycle, error handling, naming,
   dependency direction, and helper usage.
4. Check whether the change creates a parallel abstraction where a local helper
   already exists.
5. File only if the drift creates maintainability, correctness, security,
   reliability, or reviewability risk.

## Review Questions

- Does the change follow the project's package boundaries?
- Does it use established helpers instead of inventing parallel patterns?
- Does it preserve framework lifecycle hooks and error conventions?
- Does it introduce abstraction before the local framework needs it?
- Does it bypass deterministic gates, validators, or adapters?
- Does it make tests or setup use a different path than production?
- Does it create a new convention without updating framework docs?

## Technical Patterns To Check

### Package and Dependency Boundaries

- HTTP composition belongs in app/server layers, not in domain/review structs.
- External provider details should stay in adapter/tool packages.
- Review state should stay provider-neutral after normalization.
- UI/TUI rendering should not own pipeline state transitions.
- Config parsing should not reach into runtime adapters.

### Local Helper and Adapter Usage

- Reimplemented HTTP send/JSON/error handling instead of existing provider
  helper.
- New filesystem, SCM, memory, or compression path bypasses existing adapters.
- New validation logic duplicates or contradicts deterministic validators.
- Test fakes take a different contract than production interfaces.

### Lifecycle Hooks and Error Semantics

- Context cancellation not propagated through local framework patterns.
- Errors lose the local convention of dependency/run/path context.
- New background work bypasses worker queue/backpressure pattern.
- Readiness or health checks drift from required dependency semantics.

### Abstraction Discipline

- New registry/interface is added with one implementation and no real complexity
  reduction.
- Generic naming hides concrete external capabilities.
- File/folder split makes lifecycle harder to read.
- Refactor crosses ownership boundaries without behavior need.

### Testing Conventions

- Behavioral changes lack tests beside the package they affect.
- Tests depend on real network/API calls where local fakes are expected.
- Docker/config changes lack `docker compose config` or equivalent validation.
- Generated files or caches are committed instead of ignored.

## Execution Rules

Framework findings require a local rule or strong nearby convention plus a real
maintenance or production consequence. Do not file because a different structure
is personally preferred. File when the change makes the lifecycle harder to
trace, bypasses established helpers, splits ownership across packages, or creates
a new convention without updating the rule source.

Choose the smallest repair:

- move code to the package that owns the lifecycle stage
- reuse the existing helper, adapter, validator, or config path
- rename confusing files or types so concrete responsibility is visible
- update framework docs when the new convention is intentional

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Rule discovery | `filesystem`, `corpus-selector` | Load AGENTS.md, CLAUDE.md when present, local rules folders, framework docs, package READMEs, and nearby established code patterns. |
| Pattern comparison | `diff-analyzer`, `scm-api` | Compare changed code against package boundaries, helper APIs, naming, lifecycle hooks, ownership, and local test style. |
| Finding gate | `validator` | File only violations of explicit rules or strong local conventions with concrete maintenance or production impact. |

## Escalation Signals

- A change crosses package/layer boundaries that repository rules assign to another module.
- New code bypasses a local helper, validator, policy gate, or lifecycle function that exists to enforce behavior.
- Style preference is being treated as a defect without an explicit rule or risk.

## Evidence Standard

A framework finding must include:

- local rule or nearby pattern source
- changed file and package/module boundary involved
- how the change drifts from the framework
- concrete risk caused by the drift
- smallest code or docs change that restores the convention

## Runtime Integration Checks

- Verify the changed package still matches the repository's intended ownership map: app wiring, channel adapter, pipeline state, tools integration, validator, publisher, memory, or CLI/TUI.
- Confirm new helpers are reachable by the existing lifecycle and are not orphaned abstractions that the agent never selects or executes.
- For skills, enforce `skill-name/SKILL.md` loading, valid YAML frontmatter, and body instructions that explain when and how the runtime should use the skill.
- For docs or generated knowledge, require the loader/corpus selector to discover them through generic rules instead of project-specific hard-coding.

## Review Output Contract

Return framework findings only when the changed code breaks a named local rule, ownership boundary, or established helper pattern. Include the expected package/location, the actual location or bypass, and the smallest move/rename/API reuse that restores readability and connectivity.

## False Positive Checks

Do not report if:

- the project has no established rule for the area
- the change intentionally updates the convention and docs/tests move with it
- the issue is purely formatting and covered by tooling
- the new abstraction clearly removes real duplication or complexity
- the nearby pattern is obsolete and contradicted by newer instructions

## Finding Template

```text
Title: <framework rule drift>
Rule source: <AGENTS.md/rules/path/nearby file>
Changed path: <file>
Expected pattern: <local framework behavior>
Actual change: <drift>
Risk: <maintainability/correctness/security/reliability impact>
Fix: <use helper, move package, update docs, or add missing gate>
```

## Finding Rule

Prefer findings that point to a concrete local rule or established nearby
pattern. Do not report style preferences unless they affect maintainability,
correctness, or framework consistency.
