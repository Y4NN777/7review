package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerCompose_WiresRequiredSidecarsOnOneNetwork(t *testing.T) {
	data := readRepoFile(t, "docker-compose.yml")

	required := []string{
		"7review:",
		"headroom:",
		"mempalace:",
		"HEADROOM_URL: http://headroom:8787",
		"MEMPALACE_URL: http://mempalace:8788",
		"CORPUS_ROOT: /workspace",
		"HTTP_READ_HEADER_TIMEOUT_MS:",
		"HTTP_WRITE_TIMEOUT_MS:",
		"WEBHOOK_JOB_TIMEOUT_MS:",
		`test: ["CMD", "/app/7review", "healthcheck"]`,
		"${CORPUS_ROOT:-.}:/workspace:ro",
		"GITLAB_URL:",
		"review-agent:",
		"mempalace-data:",
		"OPENROUTER_API_KEY:",
		"OPENROUTER_BASE_URL:",
		"DEEPSEEK_API_KEY:",
		"DEEPSEEK_BASE_URL:",
		`PROVIDER: "${PROVIDER:-}"`,
		"PROVIDER_API_KEY:",
		"PROVIDER_BASE_URL:",
		"REVIEW_MODEL:",
		"SMALL_MODEL:",
	}
	for _, item := range required {
		if !strings.Contains(data, item) {
			t.Fatalf("docker-compose.yml missing %q", item)
		}
	}
	if strings.Count(data, "driver: bridge") != 1 {
		t.Fatalf("expected one bridge network, got compose:\n%s", data)
	}
}

func TestDockerfiles_BuildAgentAndSidecarImages(t *testing.T) {
	files := map[string][]string{
		"Dockerfile": {
			"go build",
			"/app/7review",
			"mkdir -p /out/data/7review",
			"/out/agent",
			"COPY --from=build --chown=nonroot:nonroot /out/data /data",
			"COPY --from=build --chown=nonroot:nonroot /out/agent /app/agent",
			"agent/instructions.md",
			"agent/skills",
			`HEALTHCHECK`,
			`"/app/7review", "healthcheck"`,
		},
		filepath.Join("docker", "headroom-bridge", "Dockerfile"): {
			"headroom-ai",
			"app.py",
		},
		filepath.Join("docker", "mempalace-bridge", "Dockerfile"): {
			"mempalace",
			"/data",
			"app.py",
		},
	}

	for name, required := range files {
		data := readRepoFile(t, name)
		for _, item := range required {
			if !strings.Contains(data, item) {
				t.Fatalf("%s missing %q", name, item)
			}
		}
	}
}

func TestComposeSmokeScriptExercisesFullStackReadiness(t *testing.T) {
	data := readRepoFile(t, filepath.Join("scripts", "compose_smoke.sh"))
	for _, item := range []string{
		"COMPOSE_PROJECT_NAME",
		"7review_smoke",
		"docker compose up --wait",
		"docker compose exec -T 7review /app/7review status --plain --server http://127.0.0.1:8080",
		"docker compose down -v --remove-orphans",
		"trap cleanup EXIT",
	} {
		if !strings.Contains(data, item) {
			t.Fatalf("compose smoke script missing %q", item)
		}
	}
}

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
