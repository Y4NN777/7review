# Repository Guidelines

## Project Structure & Module Organization

This repository contains a Go code-review agent for GitLab merge requests. `cmd/7review/main.go` is the CLI entrypoint. `agent/app/` wires HTTP routes and dependencies. `agent/channels/` adapts webhooks into normalized review requests. `agent/pipeline/` coordinates the workflow, with support packages for `jobs`, `policy`, `validator`, `publisher`, `memory`, and `review` state. `agent/orchestrator/` handles model-role routing and fallbacks, while `agent/llm/providers/` contains concrete LLM clients.

If you add new code, keep responsibilities separated by package intent: entrypoint code in `cmd/`, HTTP composition in `agent/app`, external adapters in `agent/channels`, workflow state in `agent/jobs` and `agent/review`, deterministic gates in `agent/policy` and `agent/validator`, and provider integrations in `agent/llm/providers`. Place tests beside the code they cover using Go's `*_test.go` convention.

## Build, Test, and Development Commands

- `gofmt -w ./cmd/7review ./agent/...`: format all Go files.
- `go test ./...`: run all tests once a valid `go.mod` and package layout are present.
- `go run ./cmd/7review`: start the webhook server locally after configuring required environment variables.
- `ORCHESTRATOR_CONFIG=./orchestrator.yaml go run ./cmd/7review`: run with the multi-provider role configuration.

The module path is `github.com/Y4NN777/7review`; keep imports under that path.

## Coding Style & Naming Conventions

Use standard Go formatting and idioms. Keep exported names descriptive, such as `BuildOrchestrator` or `review.Context`, and keep unexported helpers lower camel case, such as `getEnvInt`. Prefer small files grouped around a single concept: one provider per file, orchestration logic separate from configuration loading, and HTTP wiring separate from business logic.

## Testing Guidelines

Use Go's built-in `testing` package unless a stronger local convention is introduced. Name tests as `TestFunctionName_Behavior`, for example `TestLoadConfig_MissingGitLabToken`. Cover environment parsing, provider fallback behavior, YAML loading, and request validation. Avoid real external API calls in tests; use fake `LLMProvider` implementations or local HTTP test servers.

## Commit & Pull Request Guidelines

No usable git history is available in this working directory, so use concise imperative commit messages, for example `Add OpenAI provider fallback`. Pull requests should include a short summary, test results, configuration changes, and any screenshots or sample webhook payloads when HTTP behavior changes. Link the relevant issue or merge request when available.

## Security & Configuration Tips

Do not commit real tokens or provider API keys. Copy `.env.example` to a local ignored environment file and set `GITLAB_TOKEN`, `GITLAB_WEBHOOK_SECRET`, and the needed provider keys locally. Keep `orchestrator.yaml` free of secrets; it should contain model routing only.
