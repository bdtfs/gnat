package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadLegacyAndExpand(t *testing.T) {
	t.Parallel()
	cfg, err := func() (*Config, error) {
		return Load(writeTemp(t, `
name: legacy
scenarios:
  - name: probe
    method: GET
    url: http://example.com/x
    rps: 10
    duration: 5s
`))
	}()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Scenarios[0].IsLegacy() {
		t.Fatal("flat scenario should be legacy")
	}
	exp, err := cfg.Scenarios[0].Expand()
	if err != nil {
		t.Fatal(err)
	}
	if exp.Cfg.Type != "constant-rps" || exp.Cfg.RPS != 10 {
		t.Errorf("legacy should lift to constant-rps: %+v", exp.Cfg)
	}
	if len(exp.Flow.Steps) != 1 || exp.Flow.Steps[0].URLTmpl != "http://example.com/x" {
		t.Errorf("legacy single-step flow wrong: %+v", exp.Flow)
	}
}

func TestLoadStatefulExpandKeepsTemplates(t *testing.T) {
	t.Setenv("TC_BASE", "https://staging.example")
	cfg, err := Load(writeTemp(t, `
name: stateful
variables:
  base: "${TC_BASE}"
scenarios:
  - name: flow
    executor:
      type: ramping-vus
      stages:
        - { target: 5, duration: 10s }
        - { target: 0, duration: 5s }
    steps:
      - name: challenge
        once: true
        url: "{{base}}/api/challenge"
        extract:
          - { var: salt, from: json, path: salt, required: true }
      - name: solve
        once: true
        compute: { type: sha256-leading-zero-bits, prefix: "{{salt}}", difficulty_var: "{{difficulty}}", out: nonce }
      - name: shows
        url: "{{base}}/api/shows/{{slug}}"
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Variables["base"] != "https://staging.example" {
		t.Errorf("env not substituted: %q", cfg.Variables["base"])
	}
	if cfg.Scenarios[0].IsLegacy() {
		t.Fatal("scenario with steps must not be legacy")
	}
	exp, err := cfg.Scenarios[0].Expand()
	if err != nil {
		t.Fatal(err)
	}
	if exp.Cfg.Type != "ramping-vus" || len(exp.Cfg.Stages) != 2 {
		t.Errorf("executor wrong: %+v", exp.Cfg)
	}
	if exp.Flow.Steps[0].URLTmpl != "{{base}}/api/challenge" {
		t.Errorf("extracted-var templates must survive load unresolved: %q", exp.Flow.Steps[0].URLTmpl)
	}
	if !exp.Flow.Steps[0].Once {
		t.Error("once flag lost")
	}
	if exp.Flow.Steps[1].Compute == nil || exp.Flow.Steps[1].Compute.Out != "nonce" {
		t.Errorf("compute step not mapped: %+v", exp.Flow.Steps[1])
	}
}

func TestValidateRejectsBad(t *testing.T) {
	t.Parallel()
	bad := []string{
		`name: x` + "\n" + `scenarios: []`,
		`name: x` + "\n" + `scenarios:` + "\n" + `  - name: s` + "\n" + `    url: http://x` + "\n" + `    duration: 5s`,
		`name: x` + "\n" + `scenarios:` + "\n" + `  - name: s` + "\n" + `    executor: { type: bogus }` + "\n" + `    steps:` + "\n" + `      - { name: a, url: http://x }`,
	}
	for i, b := range bad {
		if _, err := Load(writeTemp(t, b)); err == nil {
			t.Errorf("case %d should have failed validation", i)
		}
	}
}
