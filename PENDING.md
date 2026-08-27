# Pending Work

## Current State

Detailed phase tracking now lives in `ROADMAP.md`. Keep this
file focused on the immediate queue only.

The baseline work before the channel recentering was committed in three atomic commits:

- `609d318 Add configurable input profiles`
- `57d239d Wire profiles and approval channels into runtime`
- `19afee1 Add initial async approval webhooks`
- the current channel recentering keeps Axis 2 focused on WhatsApp, Telegram, and SimpleX

Validation after these commits:

```sh
GOCACHE=/tmp/7review-go-build go test ./...
```

All tests pass.

## Axis 5 - Generalized Input Profile

Status: complete.

Implemented:

- JSON input profile schema in `schemas/input-profile.schema.json`.
- Default profile in `profiles/default.input-profile.json`.
- Profile loading, semantic validation, and compilation in `agent/profile`.
- Runtime wiring for profile-driven skills, corpus limits, path ignore policy, memory settings, publishing policy, and finding validation confidence.
- Tests for profile compilation, invalid profiles, default fallback loading, skill activation, corpus limits, path policy, and validator confidence.

## Axis 2 - Async Approval Channels

Status: implemented as a concrete runtime foundation; production provider setup remains pending.

Implemented:

- `NotificationChannel` abstraction and channel manager.
- Draft notification after review draft publication.
- Final confirmation after approved final publication.
- Explicit approval gate through channel replies.
- Supported commands:
  - `/approve <run_id>`
  - `/revise <run_id>`
  - `/suppress <run_id> <finding_id>`
- Generic internal JSON bridge:
  - `POST /channels/<channel>/inbound`
- Twilio WhatsApp provider:
  - outbound draft through Twilio Messages API
  - inbound webhook at `POST /channels/twilio/whatsapp`
  - `X-Twilio-Signature` verification
  - optional Twilio Content Template support through `content_sid`
- Telegram provider:
  - outbound draft through Telegram Bot API `sendMessage`
  - inbound webhook at `POST /channels/telegram/webhook`
  - `X-Telegram-Bot-Api-Secret-Token` verification
- SimpleX provider:
  - outbound draft through local `simplex-chat` WebSocket API
  - inbound event loop for `NewChatItems`
  - authorized sender checks by SimpleX contact id/name
- Authorized sender enforcement before any action is queued.
- Tests for provider parsing, signatures, sender authorization, and webhook routing.

Still pending before calling Axis 2 production-complete:

- Configure and test a real Twilio WhatsApp sender.
- Create and approve the WhatsApp template used for business-initiated drafts outside the 24-hour reply window.
- Expose the local 7review server through a stable HTTPS URL for provider webhooks.
- Configure Telegram `setWebhook` with a stable HTTPS URL and secret token.
- Run `simplex-chat -p 5225` on localhost, or place a secured proxy in front of it if remote.
- Run end-to-end tests with real provider callbacks:
  - draft sent to WhatsApp
  - `/approve <run_id>` from WhatsApp publishes final review
  - draft sent to Telegram
  - `/revise <run_id>` from Telegram revises the draft
  - draft sent to SimpleX
  - `/suppress <run_id> <finding_id>` from SimpleX suppresses a finding
- Add operator documentation for provider setup and troubleshooting.

## Next Axis

Next planned work after runtime packaging and Axis 2 provider setup:

Axis 1 - Stateful agent session and streaming CLI.

Before implementation, decide between:

- local WebSocket/SSE session between CLI and agent runtime
- polling over a shared state store consistent with the Headroom/MemPalace deployment model
