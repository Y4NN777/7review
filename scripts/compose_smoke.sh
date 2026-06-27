#!/usr/bin/env bash
set -euo pipefail

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-7review_smoke}"
export HTTP_PORT="${HTTP_PORT:-18080}"
export GITHUB_TOKEN="${GITHUB_TOKEN:-token}"
export GITHUB_WEBHOOK_SECRET="${GITHUB_WEBHOOK_SECRET:-secret}"
export REVIEW_API_TOKEN="${REVIEW_API_TOKEN:-agent-token}"
export OLLAMA_BASE_URL="${OLLAMA_BASE_URL:-http://ollama:11434}"
export CORPUS_ROOT="${CORPUS_ROOT:-$PWD}"
wait_timeout="${COMPOSE_WAIT_TIMEOUT:-120}"

cleanup() {
	docker compose down -v --remove-orphans
}
trap cleanup EXIT

docker compose up --wait --wait-timeout "$wait_timeout" --build
docker compose exec -T 7review /app/7review status --plain --server http://127.0.0.1:8080
