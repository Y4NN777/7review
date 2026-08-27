# Stabilization Roadmap

This roadmap tracks the work needed before 7review should be considered ready
for production-like review runs. It prioritizes stability, runtime packaging,
and end-to-end confidence before adding more approval channels or large new
features.

## Current Baseline

Status: stable local baseline.

- `main` is green with `go test ./...`.
- GitHub and GitLab review flows are implemented behind provider adapters.
- The review pipeline selects repository corpus, activates skills, recalls
  memory, reduces context, runs model review, validates findings, publishes
  draft reports, waits for human approval, publishes final reports, and writes
  approved memory.
- Approval channel foundation exists for Twilio WhatsApp, Telegram, SimpleX,
  and the generic internal JSON bridge.
- Headroom and MemPalace are external sidecars reached through HTTP clients.

## Priority Rationale

The next work should stabilize runtime and observability before expanding
features. The review pipeline already has enough moving parts: SCM enrichment,
corpus selection, skills, memory, model routing, validation, HIL approval, and
publishing. Adding more channels or agent-session features before proving
runtime packaging would increase uncertainty instead of reducing it.

Ordering principle:

1. Make the stack start reliably.
2. Prove one complete review path end-to-end.
3. Prove the implemented approval channels with real callbacks.
4. Decide durability boundaries.
5. Only then expand channels or interactive session features.

The critical path is Docker/runtime packaging first because it gives a stable
environment for every later E2E test. Provider E2E before Docker would be noisy:
failures could come from local env drift, sidecars, secrets, ports, or provider
payloads, and it would be harder to know what actually broke.

## Phase 1 - Runtime Packaging

Goal: make local and Docker startup reproducible before provider E2E work.

- Validate `docker-compose config` and the existing agent, Headroom bridge, and
  MemPalace bridge containers.
- Verify `.env.example` has every required runtime variable and no dead provider
  settings.
- Add a smoke command that starts the stack, checks readiness, and verifies that
  the sidecars respond.
- Document the exact local startup path in `docs/docker.md` and keep secrets out
  of committed config.

Exit criteria:

- `make docker-config` passes.
- Docker stack starts from a fresh checkout using documented env setup.
- Agent readiness reports configured SCM, model, Headroom, and MemPalace status.

Detailed tasks:

- Audit `Dockerfile`, `docker-compose.yml`, `.env.example`, and `Makefile` for
  drift against the current Go config loader.
- Verify bridge images build for Headroom and MemPalace without hidden local
  dependencies.
- Add or validate a smoke script that checks agent readiness and both sidecar
  health endpoints.
- Keep model/provider secrets outside compose files; only document variable
  names and safe examples.
- Record expected ports, volumes, and service DNS names.

Risks:

- `.env.example` can become stale faster than code.
- Sidecars may pass build but fail at runtime if Python dependencies drift.
- A smoke test that only checks container start is not enough; it must verify
  agent-to-sidecar connectivity.

## Phase 2 - Real Provider E2E

Goal: prove the implemented channels with real callbacks, not just unit tests.

- Configure a stable HTTPS webhook URL for local or staging tests.
- Configure Twilio WhatsApp sender and approved template.
- Configure Telegram `setWebhook` with `X-Telegram-Bot-Api-Secret-Token`.
- Run `simplex-chat -p 5225` locally or behind a secured proxy.
- Exercise draft delivery and inbound commands:
  `/approve <run_id>`, `/revise <run_id>`, and
  `/suppress <run_id> <finding_id>`.

Exit criteria:

- A draft review can be sent to each enabled provider.
- Real inbound callbacks enqueue the expected approval/revision/suppression work.
- Final publication succeeds on GitHub or GitLab after human approval.

Detailed tasks:

- Start with one SCM provider path, preferably the one with known credentials
  available locally.
- Use one small test PR/MR with a predictable diff and harmless final publish.
- Capture provider payload samples as sanitized fixtures where possible.
- Validate unauthorized sender rejection before validating happy paths.
- Verify retries are idempotent enough to avoid duplicate final publication.

Risks:

- Twilio template approval can delay WhatsApp testing.
- Telegram webhooks require a stable HTTPS URL and exact secret handling.
- SimpleX depends on a local process and should not be exposed publicly without
  an explicit secured proxy.

## Phase 3 - Durability And Operations

Goal: remove single-process assumptions that are risky for production.

- Decide whether run and queue state stay file-backed for v1 or move to a
  durable queue/store.
- Ensure accepted webhook work survives process restart or document the local
  limitation clearly.
- Add operational docs for retries, failed provider callbacks, memory sidecar
  outages, and model-provider fallback behavior.
- Keep final publication and memory writeback human-gated.

Exit criteria:

- Restart behavior is either durable or explicitly bounded and documented.
- Operator can inspect failed runs and retry safely.
- No approval channel can publish final output without explicit authorized input.

Decision needed:

- **Option A: single-instance v1.** Keep file-backed runs and bounded in-process
  queue, document restart limitations, and focus on local/staging reliability.
- **Option B: durable v1.** Add an external queue/store before production use.

Recommended default: Option A for the next milestone, unless the target runtime
requires multi-instance or restart-safe webhook acceptance immediately.

## Phase 4 - Review Quality Hardening

Goal: improve precision before expanding the product surface.

- Build a small benchmark set of real GitHub/GitLab reviews with known expected
  outcomes.
- Track true positives, false positives, missed findings, citation quality, and
  downgrade behavior for speculative items.
- Continue tightening corpus selection, source-of-truth authority handling, and
  skill coverage repair only from observed failures.

Exit criteria:

- Benchmark runs are repeatable.
- Confirmed findings cite changed code and selected source-of-truth sections.
- Speculative findings stay in draft-only or human-check sections.

Detailed tasks:

- Build fixtures from real review failures, not hypothetical edge cases.
- Track whether each finding is confirmed, likely, speculative, note, or
  question after deterministic validation.
- Keep benchmark repos/project contracts generic enough that 7review remains
  reusable across projects.
- Only tighten corpus selection when a failing example shows concrete noise or
  missed evidence.

## Not Yet

Do not start these before Phases 1-2 are stable:

- New approval channels.
- Major Docker/deployment abstractions beyond the current compose stack.
- Stateful streaming CLI/session work.
- Multi-instance horizontal scaling.
