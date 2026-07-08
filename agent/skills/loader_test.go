package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/profile"
	"github.com/Y4NN777/7review/agent/review"
)

func TestLoaderSelectFiltersProviderSpecificSkills(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "github-merge-api", "github-merge-api")
	writeSkill(t, dir, "gitlab-merge-api", "gitlab-merge-api")
	writeSkill(t, dir, "project-knowledge", "project-knowledge")

	loader := &Loader{SkillsDir: dir}
	if err := loader.Load(); err != nil {
		t.Fatal(err)
	}

	selected := loader.Select(review.Request{Provider: "github"})
	var names []string
	for _, section := range selected {
		names = append(names, section.Title)
	}

	if !contains(names, "github-merge-api") {
		t.Fatalf("expected github skill in %v", names)
	}
	if contains(names, "gitlab-merge-api") {
		t.Fatalf("did not expect gitlab skill in %v", names)
	}
	if !contains(names, "project-knowledge") {
		t.Fatalf("expected generic project knowledge skill in %v", names)
	}
}

func TestLoaderSelectUsesChangedPathsAndText(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "security-review", skillFixture("security-review", "Use for auth webhook token secret permission security changes."))
	writeCustomSkill(t, dir, "api-contract-review", skillFixture("api-contract-review", "Use for API OpenAPI response request webhook schema contract changes."))
	writeCustomSkill(t, dir, "data-migration-review", skillFixture("data-migration-review", "Use for migration database schema sql cache persistence changes."))
	writeCustomSkill(t, dir, "frontend-accessibility-review", skillFixture("frontend-accessibility-review", "Use for frontend ui css html aria accessibility changes."))
	writeCustomSkill(t, dir, "config-dependency-review", skillFixture("config-dependency-review", "Use for docker compose ci workflow workflows github yaml yml config dependency environment changes."))
	writeCustomSkill(t, dir, "test-quality-review", skillFixture("test-quality-review", "Use for tests validators parsers adapters workflows and behavior coverage."))

	loader := &Loader{SkillsDir: dir}
	if err := loader.Load(); err != nil {
		t.Fatal(err)
	}

	selected := loader.Select(review.Request{
		Title:        "Update webhook auth and OpenAPI response",
		ChangedPaths: []string{"agent/app/github.go", "docs/openapi.yaml", ".github/workflows/review.yml"},
	})
	var names []string
	for _, section := range selected {
		names = append(names, section.Title)
	}

	for _, want := range []string{"security-review", "api-contract-review", "config-dependency-review", "test-quality-review"} {
		if !contains(names, want) {
			t.Fatalf("expected %s in %v", want, names)
		}
	}
	if contains(names, "data-migration-review") {
		t.Fatalf("did not expect data migration skill in %v", names)
	}
	if contains(names, "frontend-accessibility-review") {
		t.Fatalf("did not expect frontend skill in %v", names)
	}
}

func TestLoaderSelectUsesSkillDescriptionInsteadOfHardCodedNames(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "custom-runtime-safety", `---
name: custom-runtime-safety
description: Use for worker queue timeout retry context cancellation and backpressure problems in runtime systems.
---

# Runtime Safety

## Technical Patterns

- queue backpressure
- context cancellation
- retry timeout
`)

	loader := &Loader{SkillsDir: dir}
	if err := loader.Load(); err != nil {
		t.Fatal(err)
	}
	selected := loader.Select(review.Request{
		Title:        "Fix queue timeout and context cancellation",
		ChangedPaths: []string{"agent/pipeline/workers.go"},
	})
	if len(selected) != 1 || selected[0].Title != "custom-runtime-safety" {
		t.Fatalf("expected custom skill selected, got %#v", selected)
	}
}

func TestLoaderSelectProjectWikiWhenKnowledgeBuildRequested(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "project-wiki", "project-wiki")

	loader := &Loader{SkillsDir: dir}
	if err := loader.Load(); err != nil {
		t.Fatal(err)
	}
	selected := loader.Select(review.Request{
		Title:       "Build DeepWiki context pack",
		Description: "Refresh docs/wiki architecture and project knowledge",
	})
	var names []string
	for _, section := range selected {
		names = append(names, section.Title)
	}
	if !contains(names, "project-wiki") {
		t.Fatalf("expected project-wiki in %v", names)
	}
}

func TestLoaderSelectActivationsClassifiesRequiredCoreAndProviderSkills(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "methodology-review", `---
name: methodology-review
description: Core review lifecycle.
allowed-tools: scm-api validator
metadata:
  review-domain: methodology
  risk-tier: high
---

# Methodology Review
`)
	writeCustomSkill(t, dir, "gitlab-merge-api", `---
name: gitlab-merge-api
description: GitLab merge request API operations.
allowed-tools: scm-api publisher
metadata:
  review-domain: provider-api
  risk-tier: high
---

# GitLab Merge API
`)
	writeCustomSkill(t, dir, "api-contract-review", skillFixture("api-contract-review", "Use for OpenAPI route and schema changes."))

	loader := &Loader{SkillsDir: dir}
	if err := loader.Load(); err != nil {
		t.Fatal(err)
	}

	activations := loader.SelectActivations(review.Request{
		Provider:     "gitlab",
		Title:        "Update OpenAPI schema",
		ChangedPaths: []string{"docs/openapi.yaml"},
	})
	byName := make(map[string]review.SkillActivation)
	for _, activation := range activations {
		byName[activation.Name] = activation
	}

	core := byName["methodology-review"]
	if core.Category != "core" || !core.Required || core.RiskTier != "high" || !contains(core.AllowedTools, "validator") || !contains(core.RequiredChecks, "lifecycle") {
		t.Fatalf("core activation not enriched: %#v", core)
	}
	provider := byName["gitlab-merge-api"]
	if provider.Category != "provider-api" || !provider.Required || !strings.Contains(provider.Reason, "gitlab") || !contains(provider.RequiredChecks, "scm-enrichment") {
		t.Fatalf("provider activation not enriched: %#v", provider)
	}
	triggered := byName["api-contract-review"]
	if triggered.Category != "triggered" || triggered.Required {
		t.Fatalf("triggered activation not classified: %#v", triggered)
	}
}

func TestLoaderSelectActivationsUsesInputProfile(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "custom-core", "custom-core")
	writeSkill(t, dir, "custom-github", "custom-github")
	writeCustomSkill(t, dir, "api-contract-review", skillFixture("api-contract-review", "Use for OpenAPI route and schema changes."))

	loader := &Loader{
		SkillsDir: dir,
		Profile: profile.SkillProfile{
			AlwaysOn:                  []string{"custom-core"},
			ProviderSkills:            map[string]string{"github": "custom-github"},
			TopicalActivationMinScore: 4,
		},
	}
	if err := loader.Load(); err != nil {
		t.Fatal(err)
	}

	activations := loader.SelectActivations(review.Request{
		Provider: "github",
		Title:    "OpenAPI route",
	})
	var names []string
	for _, activation := range activations {
		names = append(names, activation.Name)
	}
	if !contains(names, "custom-core") || !contains(names, "custom-github") {
		t.Fatalf("expected profile-selected core and provider skills, got %v", names)
	}
	if contains(names, "api-contract-review") {
		t.Fatalf("topical skill should require profile min score, got %v", names)
	}
}

func TestLoadSkillValidatesAnthropicStyleConstraints(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "valid-skill", "valid-skill")
	if _, err := loadSkill(filepath.Join(dir, "valid-skill", "SKILL.md")); err != nil {
		t.Fatalf("expected valid skill: %v", err)
	}

	writeSkill(t, dir, "bad-name", "Bad_Name")
	if _, err := loadSkill(filepath.Join(dir, "bad-name", "SKILL.md")); err == nil {
		t.Fatal("expected invalid name error")
	}

	writeSkill(t, dir, "different-dir", "other-skill")
	if _, err := loadSkill(filepath.Join(dir, "different-dir", "SKILL.md")); err == nil {
		t.Fatal("expected directory/name mismatch error")
	}
}

func TestLoadSkillParsesYAMLFrontmatterMetadataAndBlockDescription(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "valid-skill", `---
name: "valid-skill"
description: >-
  Use when a review needs folded YAML frontmatter,
  metadata, and activation content.
license: Apache-2.0
compatibility: "go-1.22"
allowed-tools: bash computer
metadata:
  version: "1.2.3"
  owner: "7review"
  review-domain: "loader"
  risk-tier: "medium"
---

# Valid Skill

## Activation Contract

Run this skill.
`)

	skill, err := loadSkill(filepath.Join(dir, "valid-skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "valid-skill" {
		t.Fatalf("unexpected name %q", skill.Name)
	}
	if !strings.Contains(skill.Description, "folded YAML frontmatter") || !strings.Contains(skill.Description, "activation content") {
		t.Fatalf("description was not folded correctly: %q", skill.Description)
	}
	if skill.License != "Apache-2.0" || skill.Compatibility != "go-1.22" || skill.AllowedTools != "bash computer" {
		t.Fatalf("frontmatter fields not parsed: %#v", skill)
	}
	if skill.Metadata["version"] != "1.2.3" || skill.Metadata["owner"] != "7review" || skill.Metadata["review-domain"] != "loader" || skill.Metadata["risk-tier"] != "medium" {
		t.Fatalf("metadata not parsed: %#v", skill.Metadata)
	}
	activated := skill.ActivationContent()
	for _, want := range []string{"---", `name: "valid-skill"`, "metadata:", "# Valid Skill"} {
		if !strings.Contains(activated, want) {
			t.Fatalf("activation content missing %q:\n%s", want, activated)
		}
	}
}

func TestLoaderSelectInjectsActivatedSkillWithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "security-review", `---
name: security-review
description: Use for webhook token secret security changes.
license: Apache-2.0
compatibility: go-1.22
metadata:
  version: "2.0.0"
  owner: "7review"
  review-domain: "security"
  risk-tier: "high"
---

# Security Review

## Activation Contract

Check webhook trust boundaries.
`)

	loader := &Loader{SkillsDir: dir}
	if err := loader.Load(); err != nil {
		t.Fatal(err)
	}
	selected := loader.Select(review.Request{
		Title:        "Fix webhook token validation",
		ChangedPaths: []string{"agent/app/gitlab.go"},
	})
	if len(selected) != 1 {
		t.Fatalf("expected selected skill, got %#v", selected)
	}
	for _, want := range []string{"---", "license: Apache-2.0", "metadata:", "review-domain: \"security\"", "# Security Review"} {
		if !strings.Contains(selected[0].Content, want) {
			t.Fatalf("selected skill content missing %q:\n%s", want, selected[0].Content)
		}
	}
}

func TestPriorityProductionSkillsAreEnriched(t *testing.T) {
	cases := map[string][]string{
		"security-review": {
			"review-domain: security",
			"risk-tier: high",
			"## Review Procedure",
			"## Conditional Agent Security Checklist",
			"## Secure Finding Template",
		},
		"reliability-review": {
			"review-domain: reliability",
			"risk-tier: high",
			"## Technical Patterns To Check",
			"### Context and Timeout Discipline",
			"## Evidence Standard",
		},
		"api-contract-review": {
			"review-domain: api-contract",
			"risk-tier: high",
			"### Schema Compatibility",
			"### Tool/Agent API Contracts",
			"## Evidence Standard",
		},
		"data-migration-review": {
			"review-domain: data-migration",
			"risk-tier: high",
			"### Rolling Deploy Compatibility",
			"## Safe Migration Playbook",
			"## Finding Template",
		},
		"config-dependency-review": {
			"review-domain: config-dependency",
			"risk-tier: high",
			"### Dependency and Supply Chain",
			"### Runtime Configuration",
			"## Finding Template",
		},
		"project-wiki": {
			"review-domain: project-knowledge",
			"risk-tier: medium",
			"## Output Artifacts",
			"## Context Pack Shape",
			"## Validation Checklist",
		},
		"methodology-review": {
			"review-domain: methodology",
			"risk-tier: high",
			"## Stage Checks",
			"### Event and SCM Enrichment",
			"## Anti-Patterns",
		},
		"project-knowledge": {
			"review-domain: project-knowledge",
			"risk-tier: high",
			"## Selection Algorithm",
			"## Conflict Handling",
			"## Citation Rules",
		},
		"traceability-review": {
			"review-domain: traceability",
			"risk-tier: high",
			"## Extraction Algorithm",
			"## Traceability Matrix",
			"## Severity Guidance",
		},
		"design-contract-review": {
			"review-domain: design-contract",
			"risk-tier: high",
			"## Review Algorithm",
			"### Component Boundaries",
			"## Evidence Standard",
		},
		"laws-guards-review": {
			"review-domain: laws-guards",
			"risk-tier: critical",
			"## Review Algorithm",
			"### Alternate Path Bypass",
			"## Finding Template",
		},
		"framework-rules-review": {
			"review-domain: framework-rules",
			"risk-tier: medium",
			"## Rule Resolution Order",
			"### Package and Dependency Boundaries",
			"## Evidence Standard",
		},
	}

	for name, required := range cases {
		path := filepath.Join(name, "SKILL.md")
		skill, err := loadSkill(path)
		if err != nil {
			t.Fatalf("%s should load: %v", name, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, want := range required {
			if !strings.Contains(text, want) && !strings.Contains(skill.Body, want) {
				t.Fatalf("%s missing %q", name, want)
			}
		}
	}
}

func TestAllRepositorySkillsCarryProductionMetadataAndExecutionGuidance(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			continue
		}

		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			for _, want := range []string{
				"license: Apache-2.0",
				"compatibility:",
				"allowed-tools:",
				"metadata:",
				`version: "2.0.0"`,
				"owner:",
				"review-domain:",
				"risk-tier:",
				"## Activation Contract",
				"## Tool Routing",
				"## Escalation Signals",
				"## Evidence Standard",
				"## Runtime Integration Checks",
				"## Review Output Contract",
				"## False Positive Checks",
				"## Finding Template",
			} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s missing production skill requirement %q", path, want)
				}
			}

			technicalSections := 0
			for _, heading := range []string{
				"## Review Algorithm",
				"## Review Procedure",
				"## Technical Patterns",
				"## Workflow",
				"## Stage Checks",
				"## Selection Algorithm",
				"## Extraction Algorithm",
			} {
				if strings.Contains(text, heading) {
					technicalSections++
				}
			}
			if technicalSections == 0 {
				t.Fatalf("%s needs an execution algorithm, workflow, or technical pattern section", path)
			}
			if lineCount := strings.Count(text, "\n") + 1; lineCount < 90 {
				t.Fatalf("%s is too thin for a production skill: only %d lines", path, lineCount)
			}
		})
	}
}

func TestSkillsEncodeSWEBasicsBeforeCodeMethod(t *testing.T) {
	cases := map[string][]string{
		"methodology-review": {
			"## SWE Basics Before Code Method",
			"Problem / Intent",
			"PRD",
			"SRS",
			"System Contract",
			"Responsibility Assignment",
			"Architecture / Modeling",
		},
		"design-contract-review": {
			"## SWE Basics Mapping",
			"PRD",
			"SRS",
			"System contract",
			"Invariants and constraints",
			"Responsibility assignment",
			"Architecture/modeling",
		},
		"traceability-review": {
			"## SWE Basics Trace Chain",
			"problem/use case -> PRD scope -> SRS rule",
			"system guarantee/prohibition",
			"responsible component",
			"implementation -> test",
		},
		"project-knowledge": {
			"SWE Basics chain relevance",
			"problem/PRD -> SRS -> contract/invariant",
			"responsibility -> architecture/model",
			"SWE Basics sources: problem, PRD, SRS, system contract",
			"UML/C4 or equivalent architecture model",
		},
	}

	for name, required := range cases {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(name, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			for _, want := range required {
				if !strings.Contains(text, want) {
					t.Fatalf("%s missing SWE Basics framework concept %q", name, want)
				}
			}
		})
	}
}

func writeSkill(t *testing.T, root, dirName, skillName string) {
	t.Helper()
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + skillName + "\ndescription: test skill\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeCustomSkill(t *testing.T, root, dirName, body string) {
	t.Helper()
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func skillFixture(name, description string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n\n# Test\n\n- " + description + "\n"
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
