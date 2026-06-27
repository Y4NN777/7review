# Engineer Communication

7review communicates with engineers through two channels:

- **Streaming TUI chat** for live iteration.
- **SCM comments or notes** for durable review output on the PR/MR.

## Streaming Chat

`7review chat` is for interactive work while a review is being configured,
debugged, or iterated:

```sh
go run ./cmd/7review chat
go run ./cmd/7review chat --run project!42 --server http://localhost:8080
```

The chat path is model-driven when configuration is valid. It uses the
orchestrator formatter role and streams chunks to the terminal as the provider
emits them. OpenAI-compatible providers and Ollama stream natively. Providers
without a streaming client still work through a compatibility response.

Use chat for:

- asking why readiness is failing
- checking which config is missing
- deciding the next verification command
- discussing review findings before final approval
- iterating on HIL notes before publishing memory

When `--run` is provided, chat uses the running agent's stored review state for
that PR/MR and streams Server-Sent Events from `/chat/stream?run=<id>`.

## Agent Shape

The agent has an always-on instruction file and a model-facing tool catalog:

- `agent/instructions.md`
- `agent/agent.json`
- `GET /tools`

This keeps chat flexible while still giving the model typed capabilities such as
`list_runs`, `get_run`, `stream_run_chat`, `check_ready`, and `approve_run`.

## Review Artifacts

The review pipeline still publishes durable artifacts through the configured SCM
publisher:

- draft review report
- final report after HIL approval
- finding validation status
- follow-up notes on the merge request or pull request

## HIL Gate

The human-in-the-loop gate separates draft reasoning from permanent memory:

1. The agent produces a draft.
2. Engineers discuss or correct the draft through chat and SCM comments.
3. A human approves the final report.
4. Only approved findings and notes are proposed to MemPalace.

This keeps live conversation flexible while preserving a strict audit trail for
published review results and memory writes.
