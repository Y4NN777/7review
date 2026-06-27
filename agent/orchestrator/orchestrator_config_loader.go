package orchestrator

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadOrchestratorConfigFromFile parses the repository's orchestrator.yaml
// shape into an OrchestratorConfig. It supports role blocks with primary,
// fallbacks, max_tokens, and parallel fields.
func loadOrchestratorConfigFromFile(path string) (*OrchestratorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("orchestrator config: read %s: %w", path, err)
	}

	cfg := &OrchestratorConfig{
		Roles: make(map[ModelRole]RoleConfig),
	}

	var currentRole string
	inFallbacks := false
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := stripYAMLComment(scanner.Text())
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "roles:" {
			continue
		}

		if strings.HasSuffix(trimmed, ":") && leadingSpaces(line) == 2 {
			currentRole = strings.TrimSuffix(trimmed, ":")
			cfg.Roles[ModelRole(currentRole)] = RoleConfig{MaxTokens: 2048}
			inFallbacks = false
			continue
		}
		if currentRole == "" {
			continue
		}

		roleCfg := cfg.Roles[ModelRole(currentRole)]
		switch {
		case strings.HasPrefix(trimmed, "primary:"):
			spec, err := parseModelSpec(yamlValue(trimmed, "primary:"))
			if err != nil {
				return nil, fmt.Errorf("orchestrator config: role %q primary: %w", currentRole, err)
			}
			roleCfg.Primary = spec
			inFallbacks = false
		case strings.HasPrefix(trimmed, "fallbacks:"):
			inFallbacks = true
		case inFallbacks && strings.HasPrefix(trimmed, "- "):
			spec, err := parseModelSpec(unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
			if err != nil {
				return nil, fmt.Errorf("orchestrator config: role %q fallback: %w", currentRole, err)
			}
			roleCfg.Fallbacks = append(roleCfg.Fallbacks, spec)
		case strings.HasPrefix(trimmed, "max_tokens:"):
			maxTokens, err := strconv.Atoi(yamlValue(trimmed, "max_tokens:"))
			if err != nil {
				return nil, fmt.Errorf("orchestrator config: role %q max_tokens: %w", currentRole, err)
			}
			roleCfg.MaxTokens = maxTokens
			inFallbacks = false
		case strings.HasPrefix(trimmed, "max_parallel:"):
			maxParallel, err := strconv.Atoi(yamlValue(trimmed, "max_parallel:"))
			if err != nil {
				return nil, fmt.Errorf("orchestrator config: role %q max_parallel: %w", currentRole, err)
			}
			roleCfg.MaxParallel = maxParallel
			inFallbacks = false
		case strings.HasPrefix(trimmed, "parallel:"):
			parallel, err := strconv.ParseBool(yamlValue(trimmed, "parallel:"))
			if err != nil {
				return nil, fmt.Errorf("orchestrator config: role %q parallel: %w", currentRole, err)
			}
			roleCfg.Parallel = parallel
			inFallbacks = false
		}
		cfg.Roles[ModelRole(currentRole)] = roleCfg
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("orchestrator config: scan %s: %w", path, err)
	}

	for role, roleCfg := range cfg.Roles {
		if roleCfg.Primary.Model == "" || roleCfg.Primary.Provider == "" {
			return nil, fmt.Errorf("orchestrator config: role %q missing primary", role)
		}
	}

	return cfg, nil
}

func stripYAMLComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func yamlValue(line, key string) string {
	return unquote(strings.TrimSpace(strings.TrimPrefix(line, key)))
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	return strings.Trim(value, `'`)
}

// parseModelSpec parses "model-name@provider" into a ModelSpec.
func parseModelSpec(s string) (ModelSpec, error) {
	parts := strings.SplitN(s, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ModelSpec{}, fmt.Errorf("invalid model spec %q: expected model-name@provider", s)
	}
	return ModelSpec{Model: parts[0], Provider: parts[1]}, nil
}
