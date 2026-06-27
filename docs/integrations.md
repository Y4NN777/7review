# Headroom and MemPalace Integration

Headroom and MemPalace are required production dependencies, but they are not embedded in the Go binary.

The Go service integrates with them through HTTP sidecar URLs:

- `HEADROOM_URL`
- `MEMPALACE_URL`

In local development these can point at localhost. In Docker or Kubernetes they should point at service/container DNS names on the same network, for example:

```text
HEADROOM_URL=http://headroom:8787
MEMPALACE_URL=http://mempalace:8788
```

The local Docker stack uses one private network named `review-agent` for all
three services. Only the Go agent publishes a host port; Headroom and MemPalace
are internal dependencies.

## Runtime Boundary

The Go agent owns:

- webhook validation
- SCM enrichment
- diff normalization
- skill/corpus selection
- review pipeline state
- model orchestration
- report publishing

Corpus discovery reads from `CORPUS_ROOT`. In Docker, the host path named by
`CORPUS_ROOT` is mounted read-only at `/workspace`, and the agent uses that
mount as the target repository knowledge source.

Headroom owns:

- reducing selected context before model review
- preserving section identity while shrinking payloads

MemPalace owns:

- semantic memory recall before review
- durable memory writes after final/HIL approval

No Python or TypeScript runtime is loaded into the Go process. The Docker deployment must run separate Headroom and MemPalace containers or services.

## Required Connectivity

`/ready` checks both sidecars, the run store, orchestrator wiring, and worker
queue state. It returns structured queue counters including depth, capacity,
enqueued jobs, completed jobs, and failed worker executions. If a required
dependency is unavailable, readiness returns `503`.

Review behavior is strict:

- MemPalace recall failure fails the review run.
- Headroom reduction failure fails the review run.
- MemPalace write failure fails the post-HIL path.

This is intentional for production consistency.
