---
name: design-contract-review
description: Use when implementation must be checked against explicit design intent: PRD/SRS, acceptance criteria, architecture contracts, ADRs, component ownership, API/UI/data contracts, invariants, guarantees, prohibitions, and generated project wiki sections. Finds behavior that is locally correct but violates the intended system design.
license: Apache-2.0
compatibility: Product specs, SRS/PRD, ADRs, architecture docs, API specs, UI contracts, design tokens
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: design-contract
  risk-tier: high
---

# Design Contract Review Skill

## Purpose

Use this skill when changed code must be checked against explicit design intent, not only local correctness.

## Activation Contract

Activate when changed behavior maps to a known product, architecture, API, data,
security, process, or UI contract. This skill is not for personal design taste;
it is for enforceable documented intent.

## Inputs To Prefer

- PRD problem, users, use cases, out-of-scope statements, success criteria
- SRS functional requirements, business rules, non-functional constraints, error cases
- System contracts, invariants, guarantees, prohibitions, constraints
- Responsibility maps that explain where each rule belongs
- Architecture contracts, ADRs, component maps, UML/C4/sequence/state diagrams
- API specs, event schemas, protobuf/OpenAPI/AsyncAPI
- UI contracts, design tokens, layout rules

## SWE Basics Mapping

Apply this source order before judging implementation:

1. **PRD** tells whether the changed behavior should exist and who it serves.
2. **SRS** turns intent into verifiable `MUST`, `MUST NOT`, and `SHOULD` rules.
3. **System contract** states what the system guarantees and forbids regardless
   of implementation.
4. **Invariants and constraints** define what must always remain true and what
   design walls cannot be crossed.
5. **Responsibility assignment** decides the correct owner for a rule: UI, API,
   domain model, data layer, job, integration, or deployment boundary.
6. **Architecture/modeling** confirms behavior and structure through ADRs,
   UML/C4, component maps, or equivalent docs.

If the implementation is locally correct but violates one of these layers, file
a design-contract finding. If the layer is missing and the change is high risk,
file a missing-design-source note with searched locations.

## Review Algorithm

1. Identify the design source with authority over the changed component.
2. Extract the expected behavior, invariant, boundary, or responsibility.
3. Compare the implementation path, not only the changed line.
4. Check whether tests enforce the design contract.
5. If implementation and design conflict, decide whether code or design source
   should change.
6. File findings only when the mismatch creates behavior, maintenance, safety, or
   traceability risk.

## Review Questions

- Does the implementation preserve the contract's invariants?
- Does the change alter promised behavior without updating the contract?
- Does it move responsibility across component boundaries?
- Does it add an API/UI/data behavior not represented in the design source?
- Does it satisfy the relevant requirement IDs such as `FR-*`, `NFR-*`, `INV-*`, `PRO-*`, `GAR-*`, or `ADR-*`?

## Technical Patterns To Check

### Component Boundaries

- Handler/controller taking over domain logic owned elsewhere.
- Provider-specific code leaking into provider-neutral review state.
- Model reasoning bypassing deterministic validators or gates.
- UI/TUI concerns mixed into pipeline state transitions.
- A rule that belongs in the domain/data layer is enforced only in UI.
- A business rule that belongs in the API/domain layer is hidden inside a
  background job, integration adapter, or persistence side effect.

### Contract Drift

- API or event behavior changes without updating source contract.
- Data model changes violate an invariant or guarantee.
- Security/control behavior diverges from threat model.
- Process or HIL behavior changes without updating workflow docs.
- Implementation changes a `MUST` or `MUST NOT` behavior without updating SRS.
- Code adds behavior outside PRD scope or contradicts explicit out-of-scope text.

### UX/UI Contracts

- Design tokens or layout constraints ignored by new UI.
- Accessibility or keyboard behavior contradicted by component contract.
- On-screen text describes capabilities that do not exist.
- Wizard/setup output no longer matches config loader or runtime packaging.

### Operational Contracts

- Readiness says healthy without required dependencies.
- Memory writes happen before approval.
- Publishing no longer idempotent.
- Retry behavior violates idempotency guarantees.
- A non-functional constraint such as latency, availability, privacy, audit, or
  rollback is weakened without an explicit contract change.

## Execution Rules

Use the most specific live design source as the authority. File a finding only
when a changed behavior conflicts with an explicit requirement, invariant,
ownership rule, acceptance criterion, or documented lifecycle. If the design
source is stale, do not force the old design; report the stale contract only
when the diff depends on it and the team cannot tell which behavior is intended.

Classify the resolution before emitting:

- implementation fix when code violates a current design rule
- contract update when product or architecture intent changed
- acceptance test when the behavior is intended but not protected
- exception record when the change knowingly departs from a rule

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Design source selection | `filesystem`, `corpus-selector` | Prefer explicit PRD/SRS, ADRs, design contracts, generated wiki sections, acceptance criteria, and nearby ownership rules. |
| Implementation mapping | `diff-analyzer`, `scm-api` | Map changed behavior to components, APIs, state machines, permissions, and invariants named by the design source. |
| Validation | `validator` | Keep findings only when the design source is current enough and the implementation demonstrably violates it. |

## Escalation Signals

- Code implements locally valid behavior that contradicts an explicit invariant, prohibition, or acceptance criterion.
- The diff changes a component boundary, API, lifecycle state, or ownership rule without updating the design contract.
- Multiple design sources conflict and the implementation silently chooses one.

## Evidence Standard

A design-contract finding must include:

- authoritative source path/ID
- changed implementation path
- expected design behavior
- observed mismatch
- impact if left unresolved
- whether to fix code, docs, or both

## Runtime Integration Checks

- Map each design source to the implementation responsibility it governs: UI,
  API, domain, persistence, job, integration, deployment, or operator workflow.
- Check that rules live at the layer assigned by the PRD/SRS/contract/architecture source, not only at the layer that was easiest to patch.
- If the design contract is represented in a project wiki or memory recall, require citation back to original repository sources before treating it as authoritative.
- Ensure report output separates design violations from missing/stale design documentation.

## Review Output Contract

Each design finding must name the contract clause, the changed behavior, and the violated system property. State whether the correct resolution is to change implementation, update the design source, add a missing acceptance test, or record an explicit exception.

## False Positive Checks

Do not report if:

- the design source is obsolete and replaced by a newer one
- the change updates both implementation and contract consistently
- the issue is a stylistic preference without documented authority
- the component is experimental and explicitly outside the contract

## Finding Template

```text
Title: <contract mismatch>
Contract source: <path#heading or ID>
Changed path: <file>
Expected design: <rule or invariant>
Actual implementation: <behavior introduced>
Impact: <why system behavior or ownership suffers>
Fix: <implementation change or contract update>
```

## Finding Rule

A finding should cite the changed file and the violated design source. Avoid vague design opinions; connect the issue to a concrete contract, requirement, invariant, or expected behavior.
