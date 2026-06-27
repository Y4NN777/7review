GO_FILES := $(shell rg --files -g '*.go' cmd agent)
GOCACHE ?= /tmp/7review-go-build

.PHONY: fmt test docker-config docker-build docker-smoke compose-smoke bridge-check verify

fmt:
	gofmt -w $(GO_FILES)

test:
	GOCACHE=$(GOCACHE) go test ./...

docker-config:
	docker compose config >/dev/null

docker-build:
	docker compose build

docker-smoke:
	docker run --rm \
		-e GITHUB_TOKEN=token \
		-e GITHUB_WEBHOOK_SECRET=secret \
		-e REVIEW_API_TOKEN=agent-token \
		-e HEADROOM_URL=http://headroom:8787 \
		-e MEMPALACE_URL=http://mempalace:8788 \
		-e OLLAMA_BASE_URL=http://ollama:11434 \
		7review-agent:local status --plain

compose-smoke:
	scripts/compose_smoke.sh

bridge-check:
	python3 -m py_compile docker/headroom-bridge/app.py docker/mempalace-bridge/app.py
	python3 docker/headroom-bridge/app_test.py
	python3 docker/mempalace-bridge/app_test.py

verify: fmt bridge-check test docker-config
