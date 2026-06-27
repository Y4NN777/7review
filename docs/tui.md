# TUI

The command-line interface is an operational TUI for setup and day-to-day checks.

## Commands

```sh
7review setup
7review status
7review status --server http://localhost:8080
7review tui --server http://localhost:8080
7review tui --watch --refresh 5s --server http://localhost:8080
7review history <project!mr> --server http://localhost:8080
7review history <project!mr> --type chat_message --limit 20 --server http://localhost:8080
7review chat
7review chat <project!mr> --server http://localhost:8080
7review chat --run <project!mr> --server http://localhost:8080
```

`setup` writes a local `.env` file with `0600` permissions. It asks for:

- run mode: Docker or local
- SCM provider: GitLab or GitHub
- one model provider
- Headroom and MemPalace defaults
- HTTP port and webhook worker sizing
- HTTP server timeout defaults
- corpus root for target repository knowledge when running Docker

`status` renders local config state by default. With `--server`, it calls the
agent's authenticated `/ready` endpoint and renders pipeline, orchestrator,
run-store, queue, Headroom, and MemPalace readiness. Set `REVIEW_API_TOKEN`
when the server requires operator auth.

`tui` renders the operator console as a terminal workspace: recent run activity
on the left, the current run and exact follow-up commands below it, and a compact
right rail for dependency state, queue state, provider routing, review progress,
loaded skills, and tool count. It reads live agent endpoints only: `/ready`,
`/tools`, and `/tools/execute` for `list_runs`, `get_run`,
`list_provider_status`, and `list_skills`. With `--watch`, it refreshes in place
until interrupted.

`history` renders a run timeline from `/run?id=<run-id>`, including lifecycle
events and persisted run-chat events. Use `--type` to filter one event type and
`--limit` to show only the latest matching events.

`chat` is an interactive, streaming, model-driven operator surface for setup,
status, Docker, sidecars, webhooks, review iteration, and next steps. It uses
the configured orchestrator formatter role. OpenAI-compatible providers and
Ollama stream token chunks natively; providers without a streaming client use a
single compatibility response until their streaming APIs are implemented. If
configuration is invalid, chat falls back to a local message that tells the
operator to run setup and status first.

For iterative review, pass a run ID:

```sh
7review chat project!42 --server http://localhost:8080
7review chat --run project!42 --server http://localhost:8080
```

That mode streams through the running agent's `/chat/stream` endpoint, so the
model sees the stored draft report, validated findings, status, and SCM URL for
that specific review run. Inside chat, local commands stay in the terminal flow:
`/status` checks agent readiness, `/tools` lists implemented agent tools,
`/providers` shows model provider and role routing, `/skills` lists loaded
agent skills, `/run` shows the current session summary, `/history` shows the run
timeline, and `/history chat_message 20` shows the latest persisted chat
messages without sending that command to the model. `/draft` prints the current
draft report, and `/draft final.md` writes it to a local file for review or
editing. HIL actions are explicit:
`/approve --report-file final.md` submits human approval and
`/publish-final --report-file final.md` retries final publishing for an already
approved run.

Operator HTTP commands use bounded clients: normal commands time out after 60
seconds, while streamed run chat has a longer bounded timeout for interactive
responses.
