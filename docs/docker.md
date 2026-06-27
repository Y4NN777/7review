# Docker Deployment

The production stack runs three containers on one private Compose network:

- `7review`: Go webhook server and review pipeline.
- `headroom`: HTTP bridge around Headroom context compression.
- `mempalace`: HTTP bridge around MemPalace memory recall and writes.

Only `7review` publishes a host port. Headroom and MemPalace stay private on the Compose network and are reached through:

```text
HEADROOM_URL=http://headroom:8787
MEMPALACE_URL=http://mempalace:8788
```

## Run

Create the local environment file with the setup wizard:

```sh
go run ./cmd/7review setup
```

For guided operational questions, use:

```sh
go run ./cmd/7review chat
```

Or export at least one SCM provider secret and one explicit model provider configuration, then start the stack:

```sh
export GITLAB_URL=https://gitlab.example.com
export GITLAB_TOKEN=...
export GITLAB_WEBHOOK_SECRET=...
export REVIEW_API_TOKEN=$(openssl rand -hex 32)
export OPENAI_API_KEY=...
export CORPUS_ROOT=/path/to/repository/context
docker compose up --build
```

GitHub can be used instead of GitLab by setting `GITHUB_TOKEN` and `GITHUB_WEBHOOK_SECRET`.
For local Ollama instead of a hosted model key, set `OLLAMA_BASE_URL` explicitly, for example `http://host.docker.internal:11434`.
If host port `8080` is already used, set `HTTP_PORT`, for example:

```sh
HTTP_PORT=18080 docker compose up --build
```

For a repeatable local deployment smoke test that builds the images, waits for
all three services to become healthy, checks `/ready` from inside the agent
container, and then removes the smoke stack:

```sh
make compose-smoke
```

## Repository Context Mount

`CORPUS_ROOT` is the local repository or documentation tree that 7review should
scan for review context: `AGENTS.md`, rules, PRD/SRS, ADRs, API specs, threat
models, design tokens, runbooks, and delivery docs. Compose mounts it read-only
at `/workspace` and sets the agent's internal `CORPUS_ROOT=/workspace`.

If `CORPUS_ROOT` is not set, Compose mounts the current directory. For real
reviews, point it at the target repository checkout or a prepared context-pack
directory so the model does not review with the agent image's own files.

Operator endpoints such as `/ready`, `/runs`, `/run`, `/chat/stream`,
`/approve`, `/publish/final`, and `/tools` require `Authorization: Bearer
$REVIEW_API_TOKEN` or `X-7review-Token: $REVIEW_API_TOKEN`. Webhook endpoints
remain protected by their provider-specific webhook secrets.

The HTTP server uses bounded production defaults: `HTTP_READ_HEADER_TIMEOUT_MS`,
`HTTP_READ_TIMEOUT_MS`, `HTTP_WRITE_TIMEOUT_MS`, and `HTTP_IDLE_TIMEOUT_MS`.
The defaults are suitable for webhook/API traffic while allowing long enough
streaming responses for chat.

Readiness is available at `/ready`. The response includes dependency status and
worker queue counters so operators can see backlog and failed worker executions.
In environments where host loopback is restricted, check it from inside the
Compose network:

```sh
curl -H "Authorization: Bearer $REVIEW_API_TOKEN" http://localhost:${HTTP_PORT:-8080}/ready
```

## Network Shape

There is one Docker network, `review-agent`. That is enough for this stage:

- The agent calls Headroom and MemPalace over private service DNS.
- External webhook traffic enters only through the agent's published HTTP port.
- MemPalace persists durable data in the `mempalace-data` volume.

Add separate networks later only if deployment policy requires stricter isolation, for example a public ingress network plus a private dependency network.

## Python Integration

Python is not embedded in the Go process. It exists only in sidecar images:

- `docker/headroom-bridge` installs `headroom-ai`. The `all` extra is avoided
  because it pulls GPU, benchmark, OCR, and local embedding dependencies that
  are not required by this agent's context-reduction contract.
- `docker/mempalace-bridge` installs `mempalace`.

The Go service depends only on the strict HTTP contracts documented in `docs/integrations.md`.

## Durable State

The agent persists run state, draft reports, HIL approval state, and final
reports under `MEMORY_DIR/runs`. In Docker this is mounted as the `review-data`
volume at `/data/7review`, so review iteration survives container restarts.
MemPalace keeps its own durable memory in the separate `mempalace-data` volume.
