package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type ConfigProfile struct {
	Mode string

	SCMProvider string

	GitLabURL           string
	GitLabToken         string
	GitLabWebhookSecret string

	GitHubAPIURL        string
	GitHubToken         string
	GitHubWebhookSecret string

	LLMProvider       string
	OpenAIAPIKey      string
	AnthropicAPIKey   string
	OpenRouterAPIKey  string
	OpenRouterBaseURL string
	DeepSeekAPIKey    string
	DeepSeekBaseURL   string
	MistralAPIKey     string
	GeminiAPIKey      string
	OllamaBaseURL     string

	HTTPPort            string
	CorpusRoot          string
	MemoryDir           string
	HeadroomURL         string
	MemPalaceURL        string
	HeadroomTimeoutMS   string
	MemPalaceTimeoutMS  string
	HTTPReadHeaderMS    string
	HTTPReadMS          string
	HTTPWriteMS         string
	HTTPIdleMS          string
	APIToken            string
	WebhookWorkers      string
	WebhookQueueSize    string
	OrchestratorConfig  string
	HeadroomCompression string
	MemPalaceNamespace  string
}

type SetupOptions struct {
	OutputPath string
	Force      bool
	Plain      bool
}

func DefaultConfigProfile() ConfigProfile {
	return ConfigProfile{
		Mode:                "docker",
		SCMProvider:         "gitlab",
		GitLabURL:           "",
		GitHubAPIURL:        "https://api.github.com",
		LLMProvider:         "openai",
		OpenRouterBaseURL:   "https://openrouter.ai/api",
		DeepSeekBaseURL:     "https://api.deepseek.com",
		HTTPPort:            "8080",
		CorpusRoot:          ".",
		MemoryDir:           "/data/7review",
		HeadroomURL:         "http://headroom:8787",
		MemPalaceURL:        "http://mempalace:8788",
		HeadroomTimeoutMS:   "5000",
		MemPalaceTimeoutMS:  "5000",
		HTTPReadHeaderMS:    "5000",
		HTTPReadMS:          "30000",
		HTTPWriteMS:         "120000",
		HTTPIdleMS:          "120000",
		APIToken:            "",
		WebhookWorkers:      "4",
		WebhookQueueSize:    "32",
		OrchestratorConfig:  "/app/orchestrator.yaml",
		HeadroomCompression: "0.55",
		MemPalaceNamespace:  "7review",
	}
}

func RunSetupWizard(in io.Reader, out io.Writer, opts SetupOptions) error {
	if opts.OutputPath == "" {
		opts.OutputPath = ".env"
	}
	profile := DefaultConfigProfile()
	reader := bufio.NewReader(in)

	fmt.Fprintln(out, RenderSetupIntro(opts.Plain))
	mode := ask(reader, out, "Run mode", profile.Mode)
	if mode == "local" {
		profile.Mode = "local"
		profile.HeadroomURL = "http://localhost:8787"
		profile.MemPalaceURL = "http://localhost:8788"
		profile.MemoryDir = "./.7review"
		profile.OrchestratorConfig = "./orchestrator.yaml"
	} else {
		profile.Mode = "docker"
	}

	profile.SCMProvider = normalizeChoice(ask(reader, out, "SCM provider [gitlab/github]", profile.SCMProvider), "gitlab")
	switch profile.SCMProvider {
	case "github":
		profile.GitHubAPIURL = ask(reader, out, "GitHub API URL", profile.GitHubAPIURL)
		profile.GitHubToken = askSecret(reader, out, "GitHub token")
		profile.GitHubWebhookSecret = askSecret(reader, out, "GitHub webhook secret")
	default:
		profile.SCMProvider = "gitlab"
		profile.GitLabURL = ask(reader, out, "GitLab URL", profile.GitLabURL)
		profile.GitLabToken = askSecret(reader, out, "GitLab token")
		profile.GitLabWebhookSecret = askSecret(reader, out, "GitLab webhook secret")
	}

	profile.LLMProvider = normalizeChoice(ask(reader, out, "Model provider [openai/openrouter/deepseek/anthropic/mistral/gemini/ollama]", profile.LLMProvider), "openai")
	switch profile.LLMProvider {
	case "anthropic":
		profile.AnthropicAPIKey = askSecret(reader, out, "Anthropic API key")
	case "openrouter":
		profile.OpenRouterBaseURL = ask(reader, out, "OpenRouter base URL", profile.OpenRouterBaseURL)
		profile.OpenRouterAPIKey = askSecret(reader, out, "OpenRouter API key")
	case "deepseek":
		profile.DeepSeekBaseURL = ask(reader, out, "DeepSeek base URL", profile.DeepSeekBaseURL)
		profile.DeepSeekAPIKey = askSecret(reader, out, "DeepSeek API key")
	case "mistral":
		profile.MistralAPIKey = askSecret(reader, out, "Mistral API key")
	case "gemini":
		profile.GeminiAPIKey = askSecret(reader, out, "Gemini API key")
	case "ollama":
		profile.OllamaBaseURL = ask(reader, out, "Ollama base URL", "http://host.docker.internal:11434")
	default:
		profile.LLMProvider = "openai"
		profile.OpenAIAPIKey = askSecret(reader, out, "OpenAI API key")
	}

	profile.HTTPPort = ask(reader, out, "Host HTTP port", profile.HTTPPort)
	profile.APIToken = askSecret(reader, out, "Operator API token")
	profile.WebhookWorkers = ask(reader, out, "Webhook workers", profile.WebhookWorkers)
	profile.WebhookQueueSize = ask(reader, out, "Webhook queue size", profile.WebhookQueueSize)
	profile.HTTPReadHeaderMS = ask(reader, out, "HTTP read-header timeout ms", profile.HTTPReadHeaderMS)
	profile.HTTPReadMS = ask(reader, out, "HTTP read timeout ms", profile.HTTPReadMS)
	profile.HTTPWriteMS = ask(reader, out, "HTTP write timeout ms", profile.HTTPWriteMS)
	profile.HTTPIdleMS = ask(reader, out, "HTTP idle timeout ms", profile.HTTPIdleMS)
	profile.CorpusRoot = ask(reader, out, "Corpus root for target repository context", profile.CorpusRoot)

	if err := profile.Validate(); err != nil {
		return err
	}
	if _, err := os.Stat(opts.OutputPath); err == nil && !opts.Force {
		answer := normalizeChoice(ask(reader, out, "File exists. Overwrite? [yes/no]", "no"), "no")
		if answer != "yes" && answer != "y" {
			return fmt.Errorf("setup canceled: %s already exists", opts.OutputPath)
		}
	}

	if err := os.WriteFile(opts.OutputPath, []byte(profile.EnvFile()), 0600); err != nil {
		return fmt.Errorf("write %s: %w", opts.OutputPath, err)
	}
	fmt.Fprintln(out, RenderSetupResult(opts.OutputPath, profile, opts.Plain))
	return nil
}

func (p ConfigProfile) Validate() error {
	var missing []string
	switch p.SCMProvider {
	case "github":
		if p.GitHubToken == "" {
			missing = append(missing, "GITHUB_TOKEN")
		}
		if p.GitHubWebhookSecret == "" {
			missing = append(missing, "GITHUB_WEBHOOK_SECRET")
		}
	default:
		if p.GitLabURL == "" {
			missing = append(missing, "GITLAB_URL")
		}
		if p.GitLabToken == "" {
			missing = append(missing, "GITLAB_TOKEN")
		}
		if p.GitLabWebhookSecret == "" {
			missing = append(missing, "GITLAB_WEBHOOK_SECRET")
		}
	}
	if p.HeadroomURL == "" {
		missing = append(missing, "HEADROOM_URL")
	}
	if p.MemPalaceURL == "" {
		missing = append(missing, "MEMPALACE_URL")
	}
	if p.APIToken == "" {
		missing = append(missing, "REVIEW_API_TOKEN")
	}
	if p.OpenAIAPIKey == "" && p.AnthropicAPIKey == "" && p.OpenRouterAPIKey == "" && p.DeepSeekAPIKey == "" && p.MistralAPIKey == "" && p.GeminiAPIKey == "" && p.OllamaBaseURL == "" {
		missing = append(missing, "one model provider")
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{"HTTP_PORT", p.HTTPPort},
		{"HTTP_READ_HEADER_TIMEOUT_MS", p.HTTPReadHeaderMS},
		{"HTTP_READ_TIMEOUT_MS", p.HTTPReadMS},
		{"HTTP_WRITE_TIMEOUT_MS", p.HTTPWriteMS},
		{"HTTP_IDLE_TIMEOUT_MS", p.HTTPIdleMS},
		{"HEADROOM_TIMEOUT_MS", p.HeadroomTimeoutMS},
		{"MEMPALACE_TIMEOUT_MS", p.MemPalaceTimeoutMS},
		{"WEBHOOK_WORKERS", p.WebhookWorkers},
		{"WEBHOOK_QUEUE_SIZE", p.WebhookQueueSize},
	} {
		if !positiveInteger(item.value) {
			missing = append(missing, item.name+" positive integer")
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	return nil
}

func positiveInteger(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return strings.TrimLeft(value, "0") != ""
}

func (p ConfigProfile) EnvFile() string {
	values := []struct {
		Key   string
		Value string
	}{
		{"GITLAB_URL", p.GitLabURL},
		{"GITLAB_TOKEN", p.GitLabToken},
		{"GITLAB_WEBHOOK_SECRET", p.GitLabWebhookSecret},
		{"GITHUB_API_URL", p.GitHubAPIURL},
		{"GITHUB_TOKEN", p.GitHubToken},
		{"GITHUB_WEBHOOK_SECRET", p.GitHubWebhookSecret},
		{"HTTP_PORT", p.HTTPPort},
		{"LISTEN_ADDR", ":8080"},
		{"HTTP_READ_HEADER_TIMEOUT_MS", p.HTTPReadHeaderMS},
		{"HTTP_READ_TIMEOUT_MS", p.HTTPReadMS},
		{"HTTP_WRITE_TIMEOUT_MS", p.HTTPWriteMS},
		{"HTTP_IDLE_TIMEOUT_MS", p.HTTPIdleMS},
		{"CORPUS_ROOT", p.CorpusRoot},
		{"MEMORY_DIR", p.MemoryDir},
		{"REVIEW_API_TOKEN", p.APIToken},
		{"ORCHESTRATOR_CONFIG", p.OrchestratorConfig},
		{"HEADROOM_URL", p.HeadroomURL},
		{"HEADROOM_TIMEOUT_MS", p.HeadroomTimeoutMS},
		{"HEADROOM_COMPRESSION_RATIO", p.HeadroomCompression},
		{"MEMPALACE_URL", p.MemPalaceURL},
		{"MEMPALACE_TIMEOUT_MS", p.MemPalaceTimeoutMS},
		{"MEMPALACE_NAMESPACE", p.MemPalaceNamespace},
		{"WEBHOOK_WORKERS", p.WebhookWorkers},
		{"WEBHOOK_QUEUE_SIZE", p.WebhookQueueSize},
		{"OPENAI_API_KEY", p.OpenAIAPIKey},
		{"ANTHROPIC_API_KEY", p.AnthropicAPIKey},
		{"OPENROUTER_API_KEY", p.OpenRouterAPIKey},
		{"OPENROUTER_BASE_URL", p.OpenRouterBaseURL},
		{"DEEPSEEK_API_KEY", p.DeepSeekAPIKey},
		{"DEEPSEEK_BASE_URL", p.DeepSeekBaseURL},
		{"MISTRAL_API_KEY", p.MistralAPIKey},
		{"GEMINI_API_KEY", p.GeminiAPIKey},
		{"OLLAMA_BASE_URL", p.OllamaBaseURL},
	}

	var b strings.Builder
	b.WriteString("# Generated by 7review setup\n")
	b.WriteString("# Keep this file local. It contains secrets.\n\n")
	for _, item := range values {
		fmt.Fprintf(&b, "%s=%s\n", item.Key, quoteEnv(item.Value))
	}
	return b.String()
}

func RenderSetupIntro(plain bool) string {
	text := "7review setup\nCreate a local environment file for the agent, Headroom, MemPalace, SCM, and model provider."
	if plain {
		return text
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render("7review setup")
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render("Create a local environment file for the agent, Headroom, MemPalace, SCM, and model provider.")
	return title + "\n" + body
}

func RenderSetupResult(path string, profile ConfigProfile, plain bool) string {
	lines := []string{
		fmt.Sprintf("wrote %s", path),
		fmt.Sprintf("mode: %s", profile.Mode),
		fmt.Sprintf("scm: %s", profile.SCMProvider),
		fmt.Sprintf("corpus root: %s", profile.CorpusRoot),
		fmt.Sprintf("headroom: %s", profile.HeadroomURL),
		fmt.Sprintf("mempalace: %s", profile.MemPalaceURL),
	}
	text := strings.Join(lines, "\n")
	if plain {
		return text
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(text)
}

func ask(reader *bufio.Reader, out io.Writer, label, fallback string) string {
	if fallback == "" {
		fmt.Fprintf(out, "%s: ", label)
	} else {
		fmt.Fprintf(out, "%s [%s]: ", label, fallback)
	}
	line, _ := reader.ReadString('\n')
	value := strings.TrimSpace(line)
	if value == "" {
		return fallback
	}
	return value
}

func askSecret(reader *bufio.Reader, out io.Writer, label string) string {
	return ask(reader, out, label, "")
}

func normalizeChoice(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func quoteEnv(value string) string {
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, " #\t\n\"'") {
		return strconvQuote(value)
	}
	return value
}

func strconvQuote(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}
