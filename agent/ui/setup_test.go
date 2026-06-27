package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigProfileEnvFile_DockerGitLabOpenAI(t *testing.T) {
	profile := DefaultConfigProfile()
	profile.GitLabURL = "https://gitlab.example.com"
	profile.GitLabToken = "token"
	profile.GitLabWebhookSecret = "secret"
	profile.OpenAIAPIKey = "sk-test"
	profile.APIToken = "agent-token"

	if err := profile.Validate(); err != nil {
		t.Fatal(err)
	}
	env := profile.EnvFile()
	for _, want := range []string{
		"GITLAB_URL=https://gitlab.example.com",
		"GITLAB_TOKEN=token",
		"OPENAI_API_KEY=sk-test",
		"REVIEW_API_TOKEN=agent-token",
		"HTTP_READ_HEADER_TIMEOUT_MS=5000",
		"HTTP_WRITE_TIMEOUT_MS=120000",
		"CORPUS_ROOT=.",
		"MEMORY_DIR=/data/7review",
		"HEADROOM_URL=http://headroom:8787",
		"MEMPALACE_URL=http://mempalace:8788",
		"HTTP_PORT=8080",
		"EMBEDDING_MODEL=",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("expected %q in env:\n%s", want, env)
		}
	}
}

func TestRunSetupWizard_WritesEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	input := strings.Join([]string{
		"",                           // docker
		"",                           // gitlab
		"https://gitlab.example.com", // GitLab URL
		"token",                      // GitLab token
		"secret",                     // GitLab webhook secret
		"",                           // openai
		"sk-test",                    // OpenAI key
		"",                           // embedding model
		"",                           // HTTP port
		"agent-token",                // operator API token
		"",                           // workers
		"",                           // queue size
		"",                           // read-header timeout
		"",                           // read timeout
		"",                           // write timeout
		"",                           // idle timeout
		"",                           // corpus root
		"",
	}, "\n")

	var out strings.Builder
	err := RunSetupWizard(strings.NewReader(input), &out, SetupOptions{OutputPath: path, Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "GITLAB_URL=https://gitlab.example.com") {
		t.Fatalf("unexpected env file:\n%s", string(data))
	}
	if !strings.Contains(string(data), "CORPUS_ROOT=.") {
		t.Fatalf("expected corpus root in env file:\n%s", string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600 env file permissions, got %v", info.Mode().Perm())
	}
}

func TestConfigProfileValidate_RequiresSCMAndModel(t *testing.T) {
	profile := DefaultConfigProfile()
	err := profile.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, want := range []string{"GITLAB_URL", "GITLAB_TOKEN", "GITLAB_WEBHOOK_SECRET", "REVIEW_API_TOKEN", "one model provider"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in %v", want, err)
		}
	}
}

func TestConfigProfileValidate_RejectsInvalidNumericSettings(t *testing.T) {
	profile := DefaultConfigProfile()
	profile.GitLabURL = "https://gitlab.example.com"
	profile.GitLabToken = "token"
	profile.GitLabWebhookSecret = "secret"
	profile.OpenAIAPIKey = "sk-test"
	profile.APIToken = "agent-token"
	profile.WebhookWorkers = "0"
	profile.WebhookQueueSize = "-1"
	profile.HeadroomTimeoutMS = "slow"

	err := profile.Validate()
	if err == nil {
		t.Fatal("expected invalid numeric settings error")
	}
	for _, want := range []string{"WEBHOOK_WORKERS positive integer", "WEBHOOK_QUEUE_SIZE positive integer", "HEADROOM_TIMEOUT_MS positive integer"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in %v", want, err)
		}
	}
}
