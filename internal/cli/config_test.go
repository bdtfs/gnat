package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to create a temp file with the given content and extension.
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file %s: %v", path, err)
	}
	return path
}

// --- YAML Loading ---

func TestLoadConfig_YAML_Basic(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test Suite"
variables:
  base_url: "https://example.com"
scenarios:
  - name: "GET Root"
    method: GET
    url: "{{base_url}}/"
    rps: 10
    duration: 5s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "Test Suite" {
		t.Errorf("expected name %q, got %q", "Test Suite", cfg.Name)
	}
	if len(cfg.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(cfg.Scenarios))
	}
	s := cfg.Scenarios[0]
	if s.Name != "GET Root" {
		t.Errorf("expected scenario name %q, got %q", "GET Root", s.Name)
	}
	if s.Method != "GET" {
		t.Errorf("expected method GET, got %q", s.Method)
	}
	if s.URL != "https://example.com/" {
		t.Errorf("expected URL %q, got %q", "https://example.com/", s.URL)
	}
	if s.RPS != 10 {
		t.Errorf("expected RPS 10, got %d", s.RPS)
	}
	if s.Duration != "5s" {
		t.Errorf("expected duration %q, got %q", "5s", s.Duration)
	}
}

func TestLoadConfig_YAML_YmlExtension(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yml", `
name: "YML Test"
scenarios:
  - name: "Ping"
    method: GET
    url: "https://example.com/ping"
    rps: 1
    duration: 1s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "YML Test" {
		t.Errorf("expected name %q, got %q", "YML Test", cfg.Name)
	}
}

func TestLoadConfig_YAML_WithThresholds(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Threshold Test"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    rps: 10
    duration: 5s
thresholds:
  p95_latency_ms: 100.5
  p99_latency_ms: 200
  error_rate: 0.01
  min_rps: 95
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Thresholds == nil {
		t.Fatal("expected thresholds to be set")
	}
	if cfg.Thresholds.P95LatencyMs == nil || *cfg.Thresholds.P95LatencyMs != 100.5 {
		t.Errorf("expected p95_latency_ms 100.5, got %v", cfg.Thresholds.P95LatencyMs)
	}
	if cfg.Thresholds.P99LatencyMs == nil || *cfg.Thresholds.P99LatencyMs != 200 {
		t.Errorf("expected p99_latency_ms 200, got %v", cfg.Thresholds.P99LatencyMs)
	}
	if cfg.Thresholds.ErrorRate == nil || *cfg.Thresholds.ErrorRate != 0.01 {
		t.Errorf("expected error_rate 0.01, got %v", cfg.Thresholds.ErrorRate)
	}
	if cfg.Thresholds.MinRPS == nil || *cfg.Thresholds.MinRPS != 95 {
		t.Errorf("expected min_rps 95, got %v", cfg.Thresholds.MinRPS)
	}
}

func TestLoadConfig_YAML_WithHeaders(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Headers Test"
variables:
  token: "my-secret-token"
scenarios:
  - name: "Authed Request"
    method: POST
    url: "https://example.com/api"
    headers:
      Authorization: "Bearer {{token}}"
      Content-Type: "application/json"
    body: '{"key": "value"}'
    rps: 5
    duration: 10s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := cfg.Scenarios[0]
	if s.Headers["Authorization"] != "Bearer my-secret-token" {
		t.Errorf("expected header Authorization %q, got %q", "Bearer my-secret-token", s.Headers["Authorization"])
	}
	if s.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected header Content-Type %q, got %q", "application/json", s.Headers["Content-Type"])
	}
	if s.Body != `{"key": "value"}` {
		t.Errorf("expected body %q, got %q", `{"key": "value"}`, s.Body)
	}
}

func TestLoadConfig_YAML_MultipleScenarios(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Multi-Scenario"
scenarios:
  - name: "Scenario A"
    method: GET
    url: "https://example.com/a"
    rps: 10
    duration: 5s
  - name: "Scenario B"
    method: POST
    url: "https://example.com/b"
    body: "data"
    rps: 20
    duration: 10s
  - name: "Scenario C"
    method: DELETE
    url: "https://example.com/c"
    rps: 5
    duration: 3s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Scenarios) != 3 {
		t.Fatalf("expected 3 scenarios, got %d", len(cfg.Scenarios))
	}
	if cfg.Scenarios[0].Name != "Scenario A" {
		t.Errorf("expected first scenario name %q, got %q", "Scenario A", cfg.Scenarios[0].Name)
	}
	if cfg.Scenarios[1].Method != "POST" {
		t.Errorf("expected second scenario method POST, got %q", cfg.Scenarios[1].Method)
	}
	if cfg.Scenarios[2].Method != "DELETE" {
		t.Errorf("expected third scenario method DELETE, got %q", cfg.Scenarios[2].Method)
	}
}

// --- JSON Loading ---

func TestLoadConfig_JSON_Basic(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.json", `{
  "name": "JSON Test",
  "variables": {
    "base_url": "https://example.com"
  },
  "scenarios": [
    {
      "name": "GET Root",
      "method": "GET",
      "url": "{{base_url}}/",
      "rps": 10,
      "duration": "5s"
    }
  ]
}`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "JSON Test" {
		t.Errorf("expected name %q, got %q", "JSON Test", cfg.Name)
	}
	if cfg.Scenarios[0].URL != "https://example.com/" {
		t.Errorf("expected URL %q, got %q", "https://example.com/", cfg.Scenarios[0].URL)
	}
}

func TestLoadConfig_JSON_WithThresholds(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.json", `{
  "name": "JSON Thresholds",
  "scenarios": [
    {
      "name": "Test",
      "method": "GET",
      "url": "https://example.com",
      "rps": 10,
      "duration": "5s"
    }
  ],
  "thresholds": {
    "p95_latency_ms": 100,
    "error_rate": 0.05
  }
}`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Thresholds == nil {
		t.Fatal("expected thresholds to be set")
	}
	if cfg.Thresholds.P95LatencyMs == nil || *cfg.Thresholds.P95LatencyMs != 100 {
		t.Errorf("expected p95_latency_ms 100, got %v", cfg.Thresholds.P95LatencyMs)
	}
	if cfg.Thresholds.P99LatencyMs != nil {
		t.Errorf("expected p99_latency_ms nil, got %v", *cfg.Thresholds.P99LatencyMs)
	}
}

// --- Variable Substitution ---

func TestLoadConfig_TemplateVarSubstitution(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Var Test"
variables:
  host: "api.example.com"
  version: "v2"
scenarios:
  - name: "API Call"
    method: GET
    url: "https://{{host}}/{{version}}/data"
    rps: 1
    duration: 1s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://api.example.com/v2/data"
	if cfg.Scenarios[0].URL != expected {
		t.Errorf("expected URL %q, got %q", expected, cfg.Scenarios[0].URL)
	}
}

func TestLoadConfig_EnvVarSubstitution(t *testing.T) {
	t.Setenv("GNAT_TEST_TOKEN", "secret-abc-123")

	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Env Test"
variables:
  token: "${GNAT_TEST_TOKEN}"
scenarios:
  - name: "Auth Call"
    method: GET
    url: "https://example.com/api"
    headers:
      Authorization: "Bearer {{token}}"
    rps: 1
    duration: 1s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Variables["token"] != "secret-abc-123" {
		t.Errorf("expected token %q, got %q", "secret-abc-123", cfg.Variables["token"])
	}
	if cfg.Scenarios[0].Headers["Authorization"] != "Bearer secret-abc-123" {
		t.Errorf("expected Authorization header %q, got %q",
			"Bearer secret-abc-123", cfg.Scenarios[0].Headers["Authorization"])
	}
}

func TestLoadConfig_EnvVarSubstitutionInline(t *testing.T) {
	t.Setenv("GNAT_TEST_HOST", "prod.example.com")
	t.Setenv("GNAT_TEST_PORT", "9090")

	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Inline Env"
variables:
  base_url: "https://${GNAT_TEST_HOST}:${GNAT_TEST_PORT}"
scenarios:
  - name: "Test"
    method: GET
    url: "{{base_url}}/api"
    rps: 1
    duration: 1s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedURL := "https://prod.example.com:9090/api"
	if cfg.Scenarios[0].URL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, cfg.Scenarios[0].URL)
	}
}

func TestLoadConfig_BodyVarSubstitution(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Body Var Test"
variables:
  username: "testuser"
scenarios:
  - name: "Create User"
    method: POST
    url: "https://example.com/api/users"
    body: '{"name": "{{username}}"}'
    rps: 1
    duration: 1s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"name": "testuser"}`
	if cfg.Scenarios[0].Body != expected {
		t.Errorf("expected body %q, got %q", expected, cfg.Scenarios[0].Body)
	}
}

func TestLoadConfig_UndefinedTemplateVar_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Missing Var"
scenarios:
  - name: "Test"
    method: GET
    url: "https://{{undefined_host}}/api"
    rps: 1
    duration: 1s
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for undefined template variable, got nil")
	}
	if !contains(err.Error(), "undefined_host") {
		t.Errorf("expected error to mention 'undefined_host', got: %v", err)
	}
	if !contains(err.Error(), "not defined") {
		t.Errorf("expected error to mention 'not defined', got: %v", err)
	}
}

func TestLoadConfig_UndefinedEnvVar_Error(t *testing.T) {
	// Ensure the env var is not set.
	os.Unsetenv("GNAT_NONEXISTENT_VAR_12345")

	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Missing Env"
variables:
  token: "${GNAT_NONEXISTENT_VAR_12345}"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    headers:
      Authorization: "Bearer {{token}}"
    rps: 1
    duration: 1s
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for undefined environment variable, got nil")
	}
	if !contains(err.Error(), "GNAT_NONEXISTENT_VAR_12345") {
		t.Errorf("expected error to mention env var name, got: %v", err)
	}
	if !contains(err.Error(), "not set") {
		t.Errorf("expected error to mention 'not set', got: %v", err)
	}
}

func TestLoadConfig_NoVariablesMap_TemplateVarError(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "No Vars"
scenarios:
  - name: "Test"
    method: GET
    url: "https://{{host}}/api"
    rps: 1
    duration: 1s
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for undefined template variable with no variables map")
	}
	if !contains(err.Error(), "host") {
		t.Errorf("expected error to mention 'host', got: %v", err)
	}
}

// --- Validation Errors ---

func TestLoadConfig_NoScenarios_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Empty"
scenarios: []
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for empty scenarios")
	}
	if !contains(err.Error(), "at least one scenario") {
		t.Errorf("expected error about requiring scenarios, got: %v", err)
	}
}

func TestLoadConfig_MissingName_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing scenario name")
	}
	if !contains(err.Error(), "name is required") {
		t.Errorf("expected error about missing name, got: %v", err)
	}
}

func TestLoadConfig_MissingMethod_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "No Method"
    url: "https://example.com"
    rps: 1
    duration: 1s
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing method")
	}
	if !contains(err.Error(), "method is required") {
		t.Errorf("expected error about missing method, got: %v", err)
	}
}

func TestLoadConfig_InvalidMethod_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Bad Method"
    method: INVALID
    url: "https://example.com"
    rps: 1
    duration: 1s
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
	if !contains(err.Error(), "invalid method") {
		t.Errorf("expected error about invalid method, got: %v", err)
	}
}

func TestLoadConfig_MissingURL_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "No URL"
    method: GET
    rps: 1
    duration: 1s
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if !contains(err.Error(), "url is required") {
		t.Errorf("expected error about missing URL, got: %v", err)
	}
}

func TestLoadConfig_ZeroRPS_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Zero RPS"
    method: GET
    url: "https://example.com"
    rps: 0
    duration: 1s
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for zero RPS")
	}
	if !contains(err.Error(), "rps must be a positive") {
		t.Errorf("expected error about positive RPS, got: %v", err)
	}
}

func TestLoadConfig_NegativeRPS_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Negative RPS"
    method: GET
    url: "https://example.com"
    rps: -5
    duration: 1s
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for negative RPS")
	}
	if !contains(err.Error(), "rps must be a positive") {
		t.Errorf("expected error about positive RPS, got: %v", err)
	}
}

func TestLoadConfig_MissingDuration_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "No Duration"
    method: GET
    url: "https://example.com"
    rps: 1
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing duration")
	}
	if !contains(err.Error(), "duration is required") {
		t.Errorf("expected error about missing duration, got: %v", err)
	}
}

func TestLoadConfig_InvalidDuration_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Bad Duration"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: "not-a-duration"
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
	if !contains(err.Error(), "invalid duration") {
		t.Errorf("expected error about invalid duration, got: %v", err)
	}
}

func TestLoadConfig_NegativeDuration_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Negative Duration"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: "-5s"
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for negative duration")
	}
	if !contains(err.Error(), "duration must be positive") {
		t.Errorf("expected error about positive duration, got: %v", err)
	}
}

// --- Threshold Validation ---

func TestLoadConfig_NegativeP95Threshold_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
thresholds:
  p95_latency_ms: -10
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for negative p95 threshold")
	}
	if !contains(err.Error(), "p95_latency_ms must be positive") {
		t.Errorf("expected error about positive p95, got: %v", err)
	}
}

func TestLoadConfig_NegativeP99Threshold_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
thresholds:
  p99_latency_ms: 0
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for zero p99 threshold")
	}
	if !contains(err.Error(), "p99_latency_ms must be positive") {
		t.Errorf("expected error about positive p99, got: %v", err)
	}
}

func TestLoadConfig_ErrorRateOutOfRange_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
thresholds:
  error_rate: 1.5
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for error_rate > 1")
	}
	if !contains(err.Error(), "error_rate must be between 0 and 1") {
		t.Errorf("expected error about error_rate range, got: %v", err)
	}
}

func TestLoadConfig_NegativeErrorRate_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
thresholds:
  error_rate: -0.1
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for negative error_rate")
	}
	if !contains(err.Error(), "error_rate must be between 0 and 1") {
		t.Errorf("expected error about error_rate range, got: %v", err)
	}
}

func TestLoadConfig_NegativeMinRPS_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Test"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
thresholds:
  min_rps: -5
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for negative min_rps")
	}
	if !contains(err.Error(), "min_rps must be positive") {
		t.Errorf("expected error about positive min_rps, got: %v", err)
	}
}

// --- File Handling Edge Cases ---

func TestLoadConfig_FileNotFound_Error(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/test.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !contains(err.Error(), "reading config file") {
		t.Errorf("expected error about reading file, got: %v", err)
	}
}

func TestLoadConfig_UnsupportedExtension_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.toml", `name = "test"`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
	if !contains(err.Error(), "unsupported config file extension") {
		t.Errorf("expected error about unsupported extension, got: %v", err)
	}
}

func TestLoadConfig_InvalidYAML_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Bad YAML
  [invalid: {{
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !contains(err.Error(), "parsing YAML") {
		t.Errorf("expected error about parsing YAML, got: %v", err)
	}
}

func TestLoadConfig_InvalidJSON_Error(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.json", `{"name": broken}`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !contains(err.Error(), "parsing JSON") {
		t.Errorf("expected error about parsing JSON, got: %v", err)
	}
}

// --- Method Normalization ---

func TestLoadConfig_MethodNormalization(t *testing.T) {
	methods := []struct {
		input    string
		expected string
	}{
		{"get", "GET"},
		{"post", "POST"},
		{"Put", "PUT"},
		{"delete", "DELETE"},
		{"PATCH", "PATCH"},
		{"head", "HEAD"},
		{"options", "OPTIONS"},
	}

	for _, m := range methods {
		t.Run(m.input, func(t *testing.T) {
			dir := t.TempDir()
			path := writeTempFile(t, dir, "test.yaml", `
name: "Method Test"
scenarios:
  - name: "Test"
    method: `+m.input+`
    url: "https://example.com"
    rps: 1
    duration: 1s
`)
			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Scenarios[0].Method != m.expected {
				t.Errorf("expected method %q, got %q", m.expected, cfg.Scenarios[0].Method)
			}
		})
	}
}

// --- Scenario.ParseDuration ---

func TestScenario_ParseDuration(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Duration Test"
scenarios:
  - name: "30s Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 30s
  - name: "5m Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 5m
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d1, err := cfg.Scenarios[0].ParseDuration()
	if err != nil {
		t.Fatalf("unexpected error parsing duration: %v", err)
	}
	if d1.Seconds() != 30 {
		t.Errorf("expected 30s, got %v", d1)
	}

	d2, err := cfg.Scenarios[1].ParseDuration()
	if err != nil {
		t.Fatalf("unexpected error parsing duration: %v", err)
	}
	if d2.Minutes() != 5 {
		t.Errorf("expected 5m, got %v", d2)
	}
}

// --- Config without optional fields ---

func TestLoadConfig_MinimalConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
scenarios:
  - name: "Minimal"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "" {
		t.Errorf("expected empty name, got %q", cfg.Name)
	}
	if cfg.Variables != nil && len(cfg.Variables) != 0 {
		t.Errorf("expected nil/empty variables, got %v", cfg.Variables)
	}
	if cfg.Thresholds != nil {
		t.Errorf("expected nil thresholds, got %v", cfg.Thresholds)
	}
}

func TestLoadConfig_ThresholdsPartialFields(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Partial Thresholds"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
thresholds:
  p95_latency_ms: 50
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Thresholds == nil {
		t.Fatal("expected thresholds to be set")
	}
	if cfg.Thresholds.P95LatencyMs == nil || *cfg.Thresholds.P95LatencyMs != 50 {
		t.Errorf("expected p95_latency_ms 50, got %v", cfg.Thresholds.P95LatencyMs)
	}
	if cfg.Thresholds.P99LatencyMs != nil {
		t.Errorf("expected p99_latency_ms nil, got %v", *cfg.Thresholds.P99LatencyMs)
	}
	if cfg.Thresholds.ErrorRate != nil {
		t.Errorf("expected error_rate nil, got %v", *cfg.Thresholds.ErrorRate)
	}
	if cfg.Thresholds.MinRPS != nil {
		t.Errorf("expected min_rps nil, got %v", *cfg.Thresholds.MinRPS)
	}
}

// --- Multiple variable references in one string ---

func TestLoadConfig_MultipleVarsInSingleField(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Multi Var"
variables:
  scheme: "https"
  host: "api.example.com"
  port: "8080"
  path: "api/v1"
scenarios:
  - name: "Test"
    method: GET
    url: "{{scheme}}://{{host}}:{{port}}/{{path}}/data"
    rps: 1
    duration: 1s
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://api.example.com:8080/api/v1/data"
	if cfg.Scenarios[0].URL != expected {
		t.Errorf("expected URL %q, got %q", expected, cfg.Scenarios[0].URL)
	}
}

// --- Zero-value error_rate threshold (should be valid) ---

func TestLoadConfig_ZeroErrorRate_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Zero Error Rate"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
thresholds:
  error_rate: 0
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Thresholds == nil || cfg.Thresholds.ErrorRate == nil {
		t.Fatal("expected error_rate threshold to be set")
	}
	if *cfg.Thresholds.ErrorRate != 0 {
		t.Errorf("expected error_rate 0, got %g", *cfg.Thresholds.ErrorRate)
	}
}

func TestLoadConfig_ErrorRateOne_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "test.yaml", `
name: "Max Error Rate"
scenarios:
  - name: "Test"
    method: GET
    url: "https://example.com"
    rps: 1
    duration: 1s
thresholds:
  error_rate: 1.0
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Thresholds == nil || cfg.Thresholds.ErrorRate == nil {
		t.Fatal("expected error_rate threshold to be set")
	}
	if *cfg.Thresholds.ErrorRate != 1.0 {
		t.Errorf("expected error_rate 1.0, got %g", *cfg.Thresholds.ErrorRate)
	}
}

// --- Example files should parse successfully ---

func TestLoadConfig_ExampleBasicYAML(t *testing.T) {
	// This test requires the API_TOKEN env var to be set since the example
	// references ${API_TOKEN}.
	t.Setenv("API_TOKEN", "test-token-for-example")

	// Find the examples directory relative to the test file.
	// The test runs in internal/cli/, so examples/ is ../../examples/.
	examplesDir := filepath.Join("..", "..", "examples")
	path := filepath.Join(examplesDir, "basic.yaml")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("example file not found at %s (running outside repo root?)", path)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("example basic.yaml failed to load: %v", err)
	}
	if cfg.Name != "Basic API Load Test" {
		t.Errorf("expected name %q, got %q", "Basic API Load Test", cfg.Name)
	}
	if len(cfg.Scenarios) != 2 {
		t.Errorf("expected 2 scenarios, got %d", len(cfg.Scenarios))
	}
}

func TestLoadConfig_ExampleWithThresholdsYAML(t *testing.T) {
	t.Setenv("API_TOKEN", "test-token-for-example")

	examplesDir := filepath.Join("..", "..", "examples")
	path := filepath.Join(examplesDir, "with-thresholds.yaml")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("example file not found at %s (running outside repo root?)", path)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("example with-thresholds.yaml failed to load: %v", err)
	}
	if cfg.Name != "API Performance Validation" {
		t.Errorf("expected name %q, got %q", "API Performance Validation", cfg.Name)
	}
	if len(cfg.Scenarios) != 3 {
		t.Errorf("expected 3 scenarios, got %d", len(cfg.Scenarios))
	}
	if cfg.Thresholds == nil {
		t.Fatal("expected thresholds to be set")
	}
}

// --- Helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
