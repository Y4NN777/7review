package orchestrator

import (
	"path/filepath"
	"testing"
)

func TestHarnessConfigUsesBenchmarkedLocalRouting(t *testing.T) {
	cfg, err := loadOrchestratorConfigFromFile(filepath.Join("..", "..", "orchestrator.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	reasoner := cfg.Roles[RoleReasoner]
	if reasoner.Primary != (ModelSpec{Model: "deepseek-coder-v2:16b", Provider: "ollama"}) {
		t.Fatalf("unexpected reasoner primary: %#v", reasoner.Primary)
	}
	if len(reasoner.Fallbacks) != 1 || reasoner.Fallbacks[0] != (ModelSpec{Model: "qwen2.5-coder-7b-16k:latest", Provider: "ollama"}) {
		t.Fatalf("unexpected reasoner fallbacks: %#v", reasoner.Fallbacks)
	}

	formatter := cfg.Roles[RoleFormatter]
	if formatter.Primary != (ModelSpec{Model: "qwen2.5-coder-7b-16k:latest", Provider: "ollama"}) {
		t.Fatalf("unexpected formatter primary: %#v", formatter.Primary)
	}
	if len(formatter.Fallbacks) != 1 || formatter.Fallbacks[0] != (ModelSpec{Model: "qwen2.5-coder:7b-instruct-q4_K_M", Provider: "ollama"}) {
		t.Fatalf("unexpected formatter fallbacks: %#v", formatter.Fallbacks)
	}

	embedder := cfg.Roles[RoleEmbedder]
	if embedder.Primary != (ModelSpec{Model: "nomic-embed-text:latest", Provider: "ollama"}) {
		t.Fatalf("unexpected embedder primary: %#v", embedder.Primary)
	}
}
