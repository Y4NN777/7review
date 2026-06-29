GO_FILES := $(shell rg --files -g '*.go' cmd agent)
GOCACHE ?= /tmp/7review-go-build
HTTP_PORT ?= 8080
SERVER_URL ?= http://localhost:$(HTTP_PORT)

.PHONY: setup setup-force fmt test site-install site-dev site-build site-serve docker-config docker-build docker-up docker-down docker-restart docker-logs docker-status docker-ready docker-review-gitlab docker-review-github docker-tui docker-smoke compose-smoke bridge-check verify

setup:
	go run ./cmd/7review setup

setup-force:
	go run ./cmd/7review setup --force

fmt:
	gofmt -w $(GO_FILES)

test:
	GOCACHE=$(GOCACHE) go test ./...

site-install:
	cd site && npm install

site-dev:
	cd site && npm run start

site-build:
	cd site && npm run build

site-serve:
	cd site && npm run serve

docker-config:
	docker compose config >/dev/null

docker-build:
	docker compose build

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-restart: docker-down docker-up

docker-logs:
	docker compose logs -f 7review

docker-status:
	docker compose exec -T 7review /app/7review status --plain --server http://127.0.0.1:8080

docker-ready:
	curl -fsS -H "Authorization: Bearer $(REVIEW_API_TOKEN)" "$(SERVER_URL)/ready"

docker-review-gitlab:
	docker compose exec -T 7review /app/7review review gitlab --project "$(PROJECT_ID)" --mr "$(MR)" --server http://127.0.0.1:8080

docker-review-github:
	docker compose exec -T 7review /app/7review review github --repo "$(REPO)" --pr "$(PR)" --server http://127.0.0.1:8080

docker-tui:
	docker compose exec 7review /app/7review tui --server http://127.0.0.1:8080

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
