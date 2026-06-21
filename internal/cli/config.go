// Package cli provides CLI utilities for the GNAT load testing tool,
// including YAML/JSON configuration file parsing, variable substitution,
// and validation.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents a top-level load test configuration file.
type Config struct {
	// Name is a human-readable name for this test configuration.
	Name string `yaml:"name" json:"name"`

	// Variables is a map of variable names to values used for template
	// substitution. Values may reference environment variables using
	// the ${ENV_VAR} syntax.
	Variables map[string]string `yaml:"variables" json:"variables"`

	// Scenarios is the list of test scenarios to execute.
	Scenarios []Scenario `yaml:"scenarios" json:"scenarios"`

	// Thresholds defines optional pass/fail criteria for test results.
	Thresholds *Thresholds `yaml:"thresholds,omitempty" json:"thresholds,omitempty"`
}

// Scenario represents a single load test scenario within a configuration.
type Scenario struct {
	// Name is a human-readable name for this scenario.
	Name string `yaml:"name" json:"name"`

	// Method is the HTTP method (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS).
	Method string `yaml:"method" json:"method"`

	// URL is the target URL for the load test. May contain {{variable}}
	// placeholders that will be substituted from the variables map.
	URL string `yaml:"url" json:"url"`

	// Headers is an optional map of HTTP headers to include in requests.
	// Values may contain {{variable}} placeholders.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// Body is the optional request body. May contain {{variable}} placeholders.
	Body string `yaml:"body,omitempty" json:"body,omitempty"`

	// RPS is the target requests per second.
	RPS int `yaml:"rps" json:"rps"`

	// Duration is the test duration as a Go duration string (e.g., "30s", "5m").
	Duration string `yaml:"duration" json:"duration"`
}

// validMethods is the set of valid HTTP methods.
var validMethods = map[string]bool{
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"DELETE":  true,
	"PATCH":   true,
	"HEAD":    true,
	"OPTIONS": true,
}

// templateVarPattern matches {{variable_name}} placeholders.
var templateVarPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

// envVarPattern matches ${ENV_VAR} placeholders.
var envVarPattern = regexp.MustCompile(`\$\{(\w+)\}`)

// LoadConfig reads a configuration file from disk, auto-detects the format
// (YAML or JSON) based on file extension, performs variable substitution,
// and validates the resulting configuration.
//
// Supported extensions: .yaml, .yml (YAML), .json (JSON).
// Returns an error if the file cannot be read, parsed, or validated.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))

	var cfg Config
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file extension %q: use .yaml, .yml, or .json", ext)
	}

	if err := substituteVariables(&cfg); err != nil {
		return nil, fmt.Errorf("variable substitution: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}

// substituteVariables performs two passes of variable substitution on the config:
//
//  1. Environment variable substitution: ${ENV_VAR} references in the variables
//     map values are replaced with the corresponding environment variable values.
//  2. Template variable substitution: {{var_name}} references in scenario fields
//     (URL, headers, body) are replaced with values from the variables map.
//
// Returns an error if any referenced variable is not defined.
func substituteVariables(cfg *Config) error {
	// First pass: resolve environment variables in the variables map values.
	for key, val := range cfg.Variables {
		resolved, err := resolveEnvVars(val)
		if err != nil {
			return fmt.Errorf("variable %q: %w", key, err)
		}
		cfg.Variables[key] = resolved
	}

	// Second pass: substitute template variables in scenario fields.
	for i := range cfg.Scenarios {
		s := &cfg.Scenarios[i]

		var err error
		s.URL, err = resolveTemplateVars(s.URL, cfg.Variables)
		if err != nil {
			return fmt.Errorf("scenario %q URL: %w", s.Name, err)
		}

		s.Body, err = resolveTemplateVars(s.Body, cfg.Variables)
		if err != nil {
			return fmt.Errorf("scenario %q body: %w", s.Name, err)
		}

		for hk, hv := range s.Headers {
			resolved, err := resolveTemplateVars(hv, cfg.Variables)
			if err != nil {
				return fmt.Errorf("scenario %q header %q: %w", s.Name, hk, err)
			}
			s.Headers[hk] = resolved
		}
	}

	return nil
}

// resolveEnvVars replaces all ${ENV_VAR} patterns in s with the corresponding
// environment variable values. Returns an error if an environment variable
// is referenced but not set.
func resolveEnvVars(s string) (string, error) {
	var resolveErr error
	result := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		if resolveErr != nil {
			return match
		}
		varName := envVarPattern.FindStringSubmatch(match)[1]
		val, ok := os.LookupEnv(varName)
		if !ok {
			resolveErr = fmt.Errorf("environment variable %q is not set", varName)
			return match
		}
		return val
	})
	if resolveErr != nil {
		return "", resolveErr
	}
	return result, nil
}

// resolveTemplateVars replaces all {{var_name}} patterns in s with values
// from the variables map. Returns an error if a variable is referenced
// but not defined in the map.
func resolveTemplateVars(s string, vars map[string]string) (string, error) {
	var resolveErr error
	result := templateVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		if resolveErr != nil {
			return match
		}
		varName := templateVarPattern.FindStringSubmatch(match)[1]
		val, ok := vars[varName]
		if !ok {
			resolveErr = fmt.Errorf("variable %q is not defined in the variables map", varName)
			return match
		}
		return val
	})
	if resolveErr != nil {
		return "", resolveErr
	}
	return result, nil
}

// validateConfig checks that the configuration has all required fields
// and that all values are valid.
func validateConfig(cfg *Config) error {
	if len(cfg.Scenarios) == 0 {
		return fmt.Errorf("at least one scenario is required")
	}

	for i := range cfg.Scenarios {
		if err := validateScenario(&cfg.Scenarios[i], i); err != nil {
			return err
		}
	}

	if cfg.Thresholds != nil {
		if err := validateThresholds(cfg.Thresholds); err != nil {
			return fmt.Errorf("thresholds: %w", err)
		}
	}

	return nil
}

func validateScenario(s *Scenario, i int) error {
	label := s.Name
	if label == "" {
		label = fmt.Sprintf("scenario[%d]", i)
	}

	if s.Name == "" {
		return fmt.Errorf("%s: name is required", label)
	}

	if s.Method == "" {
		return fmt.Errorf("%s: method is required", label)
	}

	upperMethod := strings.ToUpper(s.Method)
	if !validMethods[upperMethod] {
		return fmt.Errorf("%s: invalid method %q (must be one of GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)", label, s.Method)
	}
	s.Method = upperMethod

	if s.URL == "" {
		return fmt.Errorf("%s: url is required", label)
	}

	if s.RPS <= 0 {
		return fmt.Errorf("%s: rps must be a positive integer, got %d", label, s.RPS)
	}

	if s.Duration == "" {
		return fmt.Errorf("%s: duration is required", label)
	}

	d, err := time.ParseDuration(s.Duration)
	if err != nil {
		return fmt.Errorf("%s: invalid duration %q: %w", label, s.Duration, err)
	}
	if d <= 0 {
		return fmt.Errorf("%s: duration must be positive, got %s", label, s.Duration)
	}

	return nil
}

// validateThresholds checks that threshold values are within valid ranges.
func validateThresholds(t *Thresholds) error {
	if t.P95LatencyMs != nil && *t.P95LatencyMs <= 0 {
		return fmt.Errorf("p95_latency_ms must be positive, got %g", *t.P95LatencyMs)
	}
	if t.P99LatencyMs != nil && *t.P99LatencyMs <= 0 {
		return fmt.Errorf("p99_latency_ms must be positive, got %g", *t.P99LatencyMs)
	}
	if t.ErrorRate != nil && (*t.ErrorRate < 0 || *t.ErrorRate > 1) {
		return fmt.Errorf("error_rate must be between 0 and 1, got %g", *t.ErrorRate)
	}
	if t.MinRPS != nil && *t.MinRPS <= 0 {
		return fmt.Errorf("min_rps must be positive, got %g", *t.MinRPS)
	}
	return nil
}

// ParseDuration is a convenience method that parses the scenario duration string
// into a time.Duration. This is safe to call after LoadConfig has validated the config.
func (s *Scenario) ParseDuration() (time.Duration, error) {
	return time.ParseDuration(s.Duration)
}
