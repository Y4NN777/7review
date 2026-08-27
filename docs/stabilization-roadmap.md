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

## Not Yet

Do not start these before Phases 1-2 are stable:

- New approval channels.
- Major Docker/deployment abstractions beyond the current compose stack.
- Stateful streaming CLI/session work.
- Multi-instance horizontal scaling.
