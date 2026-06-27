---
name: hil-gate-review
description: Guide human-in-the-loop review decisions: decide when draft reports can be published, when final reports require approval, and how human edits affect memory.
license: Apache-2.0
compatibility: "Review-agent pipelines with draft/final reports, engineer approval, and memory writes"
allowed-tools: run-store publisher validator mempalace scm-api
metadata:
  version: "2.0.0"
  owner: "7review"
  review-domain: "human-in-the-loop"
  risk-tier: "high"
---

# HIL Gate Review Skill

## Activation Contract

Use this skill around draft generation, approval state, final publishing, rejection handling, engineer edits, chat-driven review iteration, and memory writes. Activate when code changes affect report status transitions, publisher behavior, run state, audit logs, chat commands, or memory proposal creation.

The HIL gate is a production control boundary. It must be deterministic, auditable, and independent of the LLM's preference to publish.

## Review Algorithm

1. Identify the current run state: draft, awaiting approval, approved, rejected, final-published, or failed.
2. Verify which actions are allowed from that state.
3. Confirm that model findings are validated before a draft can be exposed.
4. Confirm that final publishing requires explicit approval unless policy allows auto-final.
5. Confirm that human edits, suppressions, and additions are represented in final output.
6. Confirm that rejected findings cannot enter final reports or durable memory.
7. Confirm that final publishing is idempotent and records an audit trail.
8. Confirm that memory proposals are created only after final approval/publish semantics are satisfied.

## Gate Rules

- Draft reports may be generated automatically after deterministic validation.
- Final reports require approval when configured by policy.
- Human-rejected findings must not be written into durable memory.
- Human-added notes can become memory proposals only after final approval.
- Publishing must be idempotent: update prior bot reports rather than duplicate them.

## Technical Patterns

### State Machine

Allowed transitions should be explicit:

- `draft_generated -> awaiting_hil`
- `awaiting_hil -> approved`
- `awaiting_hil -> rejected`
- `approved -> final_published`
- `final_published -> memory_proposed`
- `rejected -> draft_generated` only when a new review run or engineer-requested revision exists

Any transition that skips validation, approval, or final publishing should be treated as a defect unless a documented policy enables it.

### Engineer Interaction

- Chat or TUI actions must map to explicit commands such as approve, reject, revise, suppress finding, add note, ask follow-up, or request rerun.
- Streaming chat may explain findings or draft changes, but it must not mutate approval state without an explicit command.
- Human edits should be attached to report metadata with author, timestamp, and reason when available.
- The agent should preserve rejected/suppressed finding IDs for audit while excluding them from final output.

### Memory Boundary

- Draft reports, speculative model reasoning, rejected findings, and unapproved chat suggestions must not be written to durable memory.
- Approved human notes can become memory proposals, not unconditional memory writes, unless policy explicitly allows direct write.
- Memory proposals should cite the run, SCM change, final report, and approving actor.

### Publishing Boundary

- Draft markers and final markers must be distinct.
- Retrying a publish step must update or supersede the existing bot report, not duplicate it.
- If HIL is required but no approval exists, the publisher should store/return "awaiting approval" instead of posting final output.

## State Ownership

HIL state must be stored in durable run state, not inferred from chat text or the latest model response.

- `drafted`: validated draft exists and may be visible to engineers as draft.
- `finalizing`: explicit approval or policy decision has been recorded and final publish is in progress.
- `finalized`: final report was published or stored with final semantics; memory proposal may now be created.
- `failed`: transition, publish, model, or adapter failure is recorded with a visible error.

The state machine should reject ambiguous commands. For example, "looks good" in chat can explain next steps, but only a structured approve command should mutate HIL approval.

## Audit Requirements

- Record approving actor, timestamp, source command/API path, and run ID when available.
- Preserve draft report, final report, selected finding IDs, suppressed finding IDs, and human notes.
- Store publish target URLs and provider IDs for draft and final reports separately.
- Store why final publishing was allowed: human approval, policy auto-final, or operator override.
- Make failed transitions visible through `/runs`, `/run`, TUI status, or logs.

## Test Expectations

- Illegal transition tests for final publish before approval.
- Idempotent approval tests for repeated approve requests.
- Rejection/suppression tests proving rejected findings are absent from final reports and memory proposals.
- Chat tests proving explanatory streaming cannot mutate approval state without explicit command routing.
- Store persistence tests proving HIL state survives process restart.

## Execution Rules

HIL findings are unsafe transition findings. File when a draft, chat response,
operator command, API call, publisher, or memory adapter can move a run into an
approved/final/durable state without the required durable approval evidence.
Suppress if policy explicitly allows auto-finalization and the run state records
that policy reason before side effects happen.

Choose the gate repair:

- endpoint auth and object-scope check for external commands
- run-store state check for approval, final publish, and memory writes
- report validation for rejected/suppressed finding exclusion
- publisher marker separation for draft vs final evidence
- audit fields for who approved, why allowed, and which artifact changed

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| State lookup | `run-store` | Read durable run state before approving, rejecting, revising, finalizing, or writing memory proposals. |
| Report validation | `validator` | Ensure only validated findings enter draft/final reports and rejected findings are excluded. |
| Publishing | `publisher`, `scm-api` | Publish draft/final artifacts with distinct markers and provider IDs. |
| Memory proposal | `mempalace` | Write memory only after final approval/publish semantics and with run/report citations. |

## Escalation Signals

- A chat message, model response, or draft report mutates approval state without a structured command.
- Final publishing can run before approval or without durable state.
- Rejected, suppressed, or speculative findings can become final output or memory.

## Evidence Standard

HIL findings must identify the illegal transition or missing control, the state before/after, and the artifact affected: draft report, final report, SCM comment, chat command, or memory proposal. Cite the code path that changes state.

## Runtime Integration Checks

- Approval, final publish, and memory write endpoints must require operator auth and load durable run state rather than trusting request body report text.
- Chat may explain or propose edits, but it must not mutate approval state, publish final reports, or write memory without deterministic endpoint execution.
- Draft, awaiting approval, approved, finalized, failed, and superseded states must be distinguishable in API, TUI, publisher, and run store behavior.
- Durable memory writes must happen after final approval/finalization policy and carry citations to report findings and source evidence.

## Review Output Contract

Describe HIL issues as unsafe state transitions. Include starting state, attempted command/API path, missing guard, side effect, and the expected deterministic gate or auth check. Avoid broad "needs human review" findings without a concrete transition.

## False Positive Checks

- Do not require HIL for every deployment if configuration explicitly sets auto-final for low-risk changes.
- Do not reject automated draft publication when policy allows drafts and the report is clearly marked draft.
- Do not require memory proposals for every final report; memory should capture durable project knowledge, not every individual finding.
- Do not block explanatory chat responses that do not mutate report, publish, or memory state.

## Review Questions

- Is this report still a draft, or has it passed HIL?
- Are rejected findings excluded from final output and memory?
- Are human edits preserved in the final report?
- Do memory updates run only after final approval?

## Finding Template

```md
### [severity] HIL gate issue

- State transition: `<from -> to>`
- Artifact: `<draft/final/comment/chat/memory>`
- Problem: `<skipped approval, unsafe mutation, duplicate publish, unapproved memory write>`
- Evidence: `<changed lines or missing guard>`
- Production impact: `<what engineers could see or persist incorrectly>`
- Suggested fix: `<state guard, audit event, idempotency check, or test>`
```
