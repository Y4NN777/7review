package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompile_ComposableInputs(t *testing.T) {
	enabled := true
	compiled, err := Compile(Profile{
		Version: 1,
		Name:    "team_profile",
		Inputs: []Input{
			{ID: "docs", Type: "corpus", Roots: []string{"."}, Include: []string{"docs/**"}, Exclude: []string{"vendor/**"}},
			{ID: "skills", Type: "skills", Directory: "./agent/skills", AlwaysOn: []string{"methodology-review"}, ProviderSkills: map[string]string{"github": "github-merge-api"}, TopicalActivationMinScore: 3},
			{ID: "ignore", Type: "path_policy", Ignore: []string{"go.sum", "node_modules/**"}},
			{ID: "memory", Type: "memory", Provider: "mempalace", Enabled: &enabled, Recall: []string{"conventions"}},
			{ID: "ops", Type: "notification_channel", Provider: "log", Enabled: &enabled, InboundToken: "secret", AuthorizedSenders: []string{"operator@example.com"}, Settings: map[string]string{"webhook_url": "https://example.com/channels/ops/inbound"}},
		},
		Policies: Policies{
			Corpus:     CorpusPolicy{MaxSections: 12, MaxSupportingSections: 2},
			Validation: ValidationPolicy{MinConfidence: 0.7},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Name != "team_profile" || compiled.Skills.Directory != "./agent/skills" {
		t.Fatalf("profile not compiled: %#v", compiled)
	}
	if compiled.Skills.TopicalActivationMinScore != 3 || compiled.Skills.ProviderSkills["github"] != "github-merge-api" {
		t.Fatalf("skills not compiled: %#v", compiled.Skills)
	}
	if compiled.Corpus.MaxSections != 12 || compiled.Corpus.MaxSupportingSections != 2 || len(compiled.Corpus.Roots) != 1 {
		t.Fatalf("corpus not compiled: %#v", compiled.Corpus)
	}
	if compiled.Validation.MinConfidence != 0.7 {
		t.Fatalf("validation not compiled: %#v", compiled.Validation)
	}
	if len(compiled.PathPolicy.Ignore) != 2 {
		t.Fatalf("path policy not compiled: %#v", compiled.PathPolicy)
	}
	if len(compiled.Channels) != 1 || compiled.Channels[0].Name != "ops" || compiled.Channels[0].InboundToken != "secret" || len(compiled.Channels[0].AuthorizedSenders) != 1 || compiled.Channels[0].Settings["webhook_url"] == "" {
		t.Fatalf("channel not compiled: %#v", compiled.Channels)
	}
}

func TestCompile_ChannelSettingsResolveEnvironment(t *testing.T) {
	t.Setenv("TWILIO_AUTH_TOKEN", "secret-token")
	enabled := true
	compiled, err := Compile(Profile{
		Version: 1,
		Name:    "team_profile",
		Inputs: []Input{{
			ID:                "operator_whatsapp",
			Type:              "notification_channel",
			Provider:          "twilio_whatsapp",
			Enabled:           &enabled,
			Settings:          map[string]string{"auth_token": "${TWILIO_AUTH_TOKEN}"},
			AuthorizedSenders: []string{"whatsapp:+33600000000"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := compiled.Channels[0].Settings["auth_token"]; got != "secret-token" {
		t.Fatalf("settings env not resolved: %q", got)
	}
}

func TestCompile_RejectsUnsupportedInput(t *testing.T) {
	_, err := Compile(Profile{Version: 1, Name: "bad", Inputs: []Input{{ID: "x", Type: "unknown"}}})
	if err == nil {
		t.Fatal("expected unsupported input error")
	}
}

func TestCompile_RejectsInvalidProfileValues(t *testing.T) {
	for name, candidate := range map[string]Profile{
		"missing_input_id": {
			Version: 1,
			Name:    "bad",
			Inputs:  []Input{{Type: "corpus"}},
		},
		"invalid_confidence": {
			Version:  1,
			Name:     "bad",
			Policies: Policies{Validation: ValidationPolicy{MinConfidence: 1.2}},
		},
		"negative_corpus_limit": {
			Version:  1,
			Name:     "bad",
			Policies: Policies{Corpus: CorpusPolicy{MaxSections: -1}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(candidate); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLoadRejectsUnknownFieldsAndLoadsDefaultProfile(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"version":1,"name":"bad","unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}

	compiled, err := Load("../../profiles/default.input-profile.json")
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Name != "default_profile" || compiled.Skills.Directory != "./agent/skills" {
		t.Fatalf("default profile not loaded: %#v", compiled)
	}
}
