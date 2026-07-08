package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Profile is the user-facing declarative input profile.
type Profile struct {
	Version  int      `json:"version"`
	Name     string   `json:"name"`
	Inputs   []Input  `json:"inputs,omitempty"`
	Policies Policies `json:"policies,omitempty"`
}

type Input struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	Roots   []string `json:"roots,omitempty"`
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`

	Directory                         string            `json:"directory,omitempty"`
	AlwaysOn                          []string          `json:"always_on,omitempty"`
	ProviderSkills                    map[string]string `json:"provider_skills,omitempty"`
	TopicalActivationMinScore         int               `json:"topical_activation_min_score,omitempty"`
	ActivateTestQualityForCodeChanges bool              `json:"activate_test_quality_for_code_changes,omitempty"`

	Provider string   `json:"provider,omitempty"`
	Enabled  *bool    `json:"enabled,omitempty"`
	Recall   []string `json:"recall,omitempty"`

	Ignore            []string          `json:"ignore,omitempty"`
	InboundToken      string            `json:"inbound_token,omitempty"`
	AuthorizedSenders []string          `json:"authorized_senders,omitempty"`
	Settings          map[string]string `json:"settings,omitempty"`
}

type Policies struct {
	Corpus     CorpusPolicy     `json:"corpus,omitempty"`
	Validation ValidationPolicy `json:"validation,omitempty"`
	Publishing PublishingPolicy `json:"publishing,omitempty"`
}

type CorpusPolicy struct {
	MaxSections           int `json:"max_sections,omitempty"`
	MaxSupportingSections int `json:"max_supporting_sections,omitempty"`
	MaxDocumentBytes      int `json:"max_document_bytes,omitempty"`
	MaxSectionBytes       int `json:"max_section_bytes,omitempty"`
}

type ValidationPolicy struct {
	MinConfidence float64 `json:"min_confidence,omitempty"`
}

type PublishingPolicy struct {
	FinalRequiresHumanApproval bool     `json:"final_requires_human_approval,omitempty"`
	InlineStrengths            []string `json:"inline_strengths,omitempty"`
	DraftOnlyStrengths         []string `json:"draft_only_strengths,omitempty"`
}

// CompiledProfile is the runtime shape consumed by packages that already own
// review behavior. It is deliberately smaller than Profile.
type CompiledProfile struct {
	Name       string
	Skills     SkillProfile
	Corpus     CorpusProfile
	PathPolicy PathPolicy
	Validation ValidationProfile
	Publishing PublishingProfile
	Memory     MemoryProfile
	Channels   []ChannelProfile
}

type SkillProfile struct {
	Directory                         string
	AlwaysOn                          []string
	ProviderSkills                    map[string]string
	TopicalActivationMinScore         int
	ActivateTestQualityForCodeChanges bool
}

type CorpusProfile struct {
	Roots                 []CorpusRoot
	MaxSections           int
	MaxSupportingSections int
	MaxDocumentBytes      int
	MaxSectionBytes       int
}

type CorpusRoot struct {
	Path    string
	Include []string
	Exclude []string
}

type PathPolicy struct {
	Ignore []string
}

type ValidationProfile struct {
	MinConfidence float64
}

type PublishingProfile struct {
	FinalRequiresHumanApproval bool
	InlineStrengths            []string
	DraftOnlyStrengths         []string
}

type MemoryProfile struct {
	Enabled  bool
	Provider string
	Recall   []string
}

type ChannelProfile struct {
	Name              string
	Provider          string
	Enabled           bool
	InboundToken      string
	AuthorizedSenders []string
	Settings          map[string]string
}

func Load(path string) (*CompiledProfile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("profile: read %s: %w", path, err)
	}
	var p Profile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return nil, fmt.Errorf("profile: decode %s: %w", path, err)
	}
	compiled, err := Compile(p)
	if err != nil {
		return nil, fmt.Errorf("profile: compile %s: %w", path, err)
	}
	return compiled, nil
}

func Compile(p Profile) (*CompiledProfile, error) {
	if err := validate(p); err != nil {
		return nil, err
	}
	out := &CompiledProfile{
		Name: strings.TrimSpace(p.Name),
		Skills: SkillProfile{
			ProviderSkills: map[string]string{},
		},
	}
	out.Corpus.MaxSections = p.Policies.Corpus.MaxSections
	out.Corpus.MaxSupportingSections = p.Policies.Corpus.MaxSupportingSections
	out.Corpus.MaxDocumentBytes = p.Policies.Corpus.MaxDocumentBytes
	out.Corpus.MaxSectionBytes = p.Policies.Corpus.MaxSectionBytes
	out.Validation.MinConfidence = p.Policies.Validation.MinConfidence
	out.Publishing.FinalRequiresHumanApproval = p.Policies.Publishing.FinalRequiresHumanApproval
	out.Publishing.InlineStrengths = cleanList(p.Policies.Publishing.InlineStrengths)
	out.Publishing.DraftOnlyStrengths = cleanList(p.Policies.Publishing.DraftOnlyStrengths)

	for _, input := range p.Inputs {
		inputType := strings.ToLower(strings.TrimSpace(input.Type))
		if inputType == "" {
			return nil, fmt.Errorf("input %q type is required", input.ID)
		}
		switch inputType {
		case "corpus":
			roots := input.Roots
			if len(roots) == 0 {
				roots = []string{"."}
			}
			for _, root := range roots {
				root = filepath.ToSlash(strings.TrimSpace(root))
				if root == "" {
					continue
				}
				out.Corpus.Roots = append(out.Corpus.Roots, CorpusRoot{
					Path:    root,
					Include: cleanList(input.Include),
					Exclude: cleanList(input.Exclude),
				})
			}
		case "skills":
			out.Skills.Directory = strings.TrimSpace(input.Directory)
			out.Skills.AlwaysOn = cleanList(input.AlwaysOn)
			out.Skills.TopicalActivationMinScore = input.TopicalActivationMinScore
			out.Skills.ActivateTestQualityForCodeChanges = input.ActivateTestQualityForCodeChanges
			for provider, skill := range input.ProviderSkills {
				provider = strings.ToLower(strings.TrimSpace(provider))
				skill = strings.TrimSpace(skill)
				if provider != "" && skill != "" {
					out.Skills.ProviderSkills[provider] = skill
				}
			}
		case "memory":
			out.Memory.Provider = strings.TrimSpace(input.Provider)
			out.Memory.Enabled = input.Enabled == nil || *input.Enabled
			out.Memory.Recall = cleanList(input.Recall)
		case "path_policy":
			out.PathPolicy.Ignore = append(out.PathPolicy.Ignore, cleanList(input.Ignore)...)
			out.PathPolicy.Ignore = append(out.PathPolicy.Ignore, cleanList(input.Exclude)...)
		case "notification_channel":
			enabled := input.Enabled == nil || *input.Enabled
			out.Channels = append(out.Channels, ChannelProfile{
				Name:              strings.TrimSpace(input.ID),
				Provider:          strings.TrimSpace(input.Provider),
				Enabled:           enabled,
				InboundToken:      strings.TrimSpace(input.InboundToken),
				AuthorizedSenders: cleanList(input.AuthorizedSenders),
				Settings:          resolveSettings(input.Settings),
			})
		default:
			return nil, fmt.Errorf("input %q has unsupported type %q", input.ID, input.Type)
		}
	}
	out.PathPolicy.Ignore = uniqueList(out.PathPolicy.Ignore)
	return out, nil
}

func validate(p Profile) error {
	if p.Version != 1 {
		return fmt.Errorf("unsupported profile version %d", p.Version)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if err := validatePositive("policies.corpus.max_sections", p.Policies.Corpus.MaxSections); err != nil {
		return err
	}
	if err := validatePositive("policies.corpus.max_supporting_sections", p.Policies.Corpus.MaxSupportingSections); err != nil {
		return err
	}
	if err := validatePositive("policies.corpus.max_document_bytes", p.Policies.Corpus.MaxDocumentBytes); err != nil {
		return err
	}
	if err := validatePositive("policies.corpus.max_section_bytes", p.Policies.Corpus.MaxSectionBytes); err != nil {
		return err
	}
	if p.Policies.Validation.MinConfidence < 0 || p.Policies.Validation.MinConfidence > 1 {
		return fmt.Errorf("policies.validation.min_confidence must be between 0 and 1")
	}
	for i, input := range p.Inputs {
		prefix := fmt.Sprintf("inputs[%d]", i)
		if strings.TrimSpace(input.ID) == "" {
			return fmt.Errorf("%s.id is required", prefix)
		}
		if strings.TrimSpace(input.Type) == "" {
			return fmt.Errorf("%s.type is required", prefix)
		}
		if input.TopicalActivationMinScore < 0 {
			return fmt.Errorf("%s.topical_activation_min_score must be greater than or equal to zero", prefix)
		}
	}
	return nil
}

func validatePositive(name string, value int) error {
	if value < 0 {
		return fmt.Errorf("%s must be greater than zero when set", name)
	}
	return nil
}

func cleanList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return uniqueList(out)
}

func uniqueList(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func resolveSettings(settings map[string]string) map[string]string {
	if len(settings) == 0 {
		return nil
	}
	out := make(map[string]string, len(settings))
	for key, value := range settings {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = resolveSettingValue(strings.TrimSpace(value))
	}
	return out
}

func resolveSettingValue(value string) string {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return os.Getenv(strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}"))
	}
	return value
}
