# Pending Work

## Current State

The current work has been committed in three atomic commits:

- `609d318 Add configurable input profiles`
- `57d239d Wire profiles and approval channels into runtime`
- `19afee1 Add WhatsApp and email approval webhooks`

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
- SendGrid email provider:
  - outbound draft through SendGrid Mail Send API
  - inbound webhook at `POST /channels/sendgrid/inbound`
  - OAuth bearer token support
  - signed webhook support through SendGrid ECDSA public key
- Mailgun email provider:
  - outbound draft through Mailgun Messages API
  - inbound webhook at `POST /channels/mailgun/inbound`
  - timestamp/token/signature verification
- Authorized sender enforcement before any action is queued.
- Tests for provider parsing, signatures, sender authorization, and webhook routing.

Still pending before calling Axis 2 production-complete:

- Configure and test a real Twilio WhatsApp sender.
- Create and approve the WhatsApp template used for business-initiated drafts outside the 24-hour reply window.
- Expose the local 7review server through a stable HTTPS URL for provider webhooks.
- Configure real SendGrid Inbound Parse or Mailgun Routes.
- Configure DNS/MX records for the selected email provider.
- Run end-to-end tests with real provider callbacks:
  - draft sent to WhatsApp
  - `/approve <run_id>` from WhatsApp publishes final review
  - draft sent by email
  - `/revise <run_id>` from email revises the draft
  - `/suppress <run_id> <finding_id>` from email suppresses a finding
- Add operator documentation for provider setup and troubleshooting.

## Next Axis

Next planned work after Axis 2 provider setup:

Axis 1 - Stateful agent session and streaming CLI.

Before implementation, decide between:

- local WebSocket/SSE session between CLI and agent runtime
- polling over a shared state store consistent with the Headroom/MemPalace deployment model

