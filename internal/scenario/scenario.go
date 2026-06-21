package scenario

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/bdtfs/gnat/internal/checks"
	"github.com/bdtfs/gnat/internal/cli"
	"github.com/bdtfs/gnat/internal/executor"
	"github.com/bdtfs/gnat/internal/extract"
	"github.com/bdtfs/gnat/internal/vu"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Name       string            `yaml:"name"`
	Variables  map[string]string `yaml:"variables,omitempty"`
	Scenarios  []Scenario        `yaml:"scenarios"`
	Thresholds *cli.Thresholds   `yaml:"thresholds,omitempty"`
}

type Scenario struct {
	Name     string            `yaml:"name"`
	Method   string            `yaml:"method,omitempty"`
	URL      string            `yaml:"url,omitempty"`
	Body     string            `yaml:"body,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	RPS      int               `yaml:"rps,omitempty"`
	Duration string            `yaml:"duration,omitempty"`
	Identity map[string]string `yaml:"identity,omitempty"`
	Weight   int               `yaml:"weight,omitempty"`
	Executor *Executor         `yaml:"executor,omitempty"`
	Steps    []Step            `yaml:"steps,omitempty"`
}

type Step struct {
	Name         string            `yaml:"name"`
	Method       string            `yaml:"method,omitempty"`
	URL          string            `yaml:"url,omitempty"`
	Body         string            `yaml:"body,omitempty"`
	Headers      map[string]string `yaml:"headers,omitempty"`
	ReadBytesCap int64             `yaml:"read_bytes_cap,omitempty"`
	Once         bool              `yaml:"once,omitempty"`
	Compute      *Compute          `yaml:"compute,omitempty"`
	Extract      []ExtractSpec     `yaml:"extract,omitempty"`
	Check        *Check            `yaml:"check,omitempty"`
}

type Compute struct {
	Type          string `yaml:"type"`
	Prefix        string `yaml:"prefix"`
	Difficulty    int    `yaml:"difficulty,omitempty"`
	DifficultyVar string `yaml:"difficulty_var,omitempty"`
	Separator     string `yaml:"separator,omitempty"`
	MaxIters      uint64 `yaml:"max_iters,omitempty"`
	Timeout       string `yaml:"timeout,omitempty"`
	Out           string `yaml:"out"`
}

type ExtractSpec struct {
	Var      string `yaml:"var"`
	From     string `yaml:"from"`
	Path     string `yaml:"path,omitempty"`
	Default  string `yaml:"default,omitempty"`
	Required bool   `yaml:"required,omitempty"`
}

type Check struct {
	ExpectStatus      []int   `yaml:"expect_status,omitempty"`
	ExpectStatusRange []int   `yaml:"expect_status_range,omitempty"`
	MaxDurationMs     float64 `yaml:"max_duration_ms,omitempty"`
	MaxTTFBMs         float64 `yaml:"max_ttfb_ms,omitempty"`
	BodyContains      string  `yaml:"body_contains,omitempty"`
	Required          bool    `yaml:"required,omitempty"`
}

type Executor struct {
	Type         string     `yaml:"type"`
	VUs          int        `yaml:"vus,omitempty"`
	RPS          int        `yaml:"rps,omitempty"`
	Duration     string     `yaml:"duration,omitempty"`
	Stages       []StageCfg `yaml:"stages,omitempty"`
	StartVUs     int        `yaml:"start_vus,omitempty"`
	MaxVUs       int        `yaml:"max_vus,omitempty"`
	GracefulStop string     `yaml:"graceful_stop,omitempty"`
}

type StageCfg struct {
	Target   int    `yaml:"target"`
	Duration string `yaml:"duration"`
}

type Expanded struct {
	Name     string
	Flow     vu.Flow
	Cfg      executor.VUConfig
	Identity map[string]string
	Weight   int
}

var envPattern = regexp.MustCompile(`\$\{(\w+)\}`)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	for k, v := range cfg.Variables {
		resolved, err := resolveEnv(v)
		if err != nil {
			return nil, fmt.Errorf("variable %q: %w", k, err)
		}
		cfg.Variables[k] = resolved
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func resolveEnv(s string) (string, error) {
	var rErr error
	out := envPattern.ReplaceAllStringFunc(s, func(m string) string {
		if rErr != nil {
			return m
		}
		name := envPattern.FindStringSubmatch(m)[1]
		val, ok := os.LookupEnv(name)
		if !ok {
			rErr = fmt.Errorf("environment variable %q is not set", name)
			return m
		}
		return val
	})
	return out, rErr
}

func (s *Scenario) IsLegacy() bool {
	return len(s.Steps) == 0 && s.Executor == nil
}

func (c *Config) Validate() error {
	if len(c.Scenarios) == 0 {
		return fmt.Errorf("at least one scenario is required")
	}
	for i := range c.Scenarios {
		s := &c.Scenarios[i]
		label := s.Name
		if label == "" {
			label = fmt.Sprintf("scenario[%d]", i)
		}
		if s.Name == "" {
			return fmt.Errorf("%s: name is required", label)
		}
		if s.IsLegacy() {
			if err := validateLegacy(s, label); err != nil {
				return err
			}
			continue
		}
		if err := validateStateful(s, label); err != nil {
			return err
		}
	}
	return nil
}

func validateLegacy(s *Scenario, label string) error {
	if s.URL == "" {
		return fmt.Errorf("%s: url is required", label)
	}
	if s.Method == "" {
		s.Method = "GET"
	}
	if s.RPS <= 0 {
		return fmt.Errorf("%s: rps must be positive", label)
	}
	if s.Duration == "" {
		return fmt.Errorf("%s: duration is required", label)
	}
	if _, err := time.ParseDuration(s.Duration); err != nil {
		return fmt.Errorf("%s: invalid duration %q: %w", label, s.Duration, err)
	}
	return nil
}

func validateStateful(s *Scenario, label string) error {
	if len(s.Steps) == 0 {
		return fmt.Errorf("%s: at least one step is required", label)
	}
	if s.Executor == nil {
		return fmt.Errorf("%s: executor is required", label)
	}
	for i := range s.Steps {
		st := &s.Steps[i]
		if st.Name == "" {
			return fmt.Errorf("%s: step[%d] name is required", label, i)
		}
		if st.Compute == nil {
			if st.URL == "" {
				return fmt.Errorf("%s/%s: url is required", label, st.Name)
			}
			if st.Method == "" {
				st.Method = "GET"
			}
		} else {
			if st.Compute.Out == "" {
				return fmt.Errorf("%s/%s: compute.out is required", label, st.Name)
			}
		}
	}
	return validateExecutor(s.Executor, label)
}

func validateExecutor(e *Executor, label string) error {
	switch e.Type {
	case "constant-rps":
		if e.RPS <= 0 {
			return fmt.Errorf("%s: executor rps must be positive", label)
		}
		if _, err := time.ParseDuration(e.Duration); err != nil {
			return fmt.Errorf("%s: invalid executor duration: %w", label, err)
		}
	case "constant-vus":
		if e.VUs <= 0 {
			return fmt.Errorf("%s: executor vus must be positive", label)
		}
		if _, err := time.ParseDuration(e.Duration); err != nil {
			return fmt.Errorf("%s: invalid executor duration: %w", label, err)
		}
	case "ramping-vus":
		if len(e.Stages) == 0 {
			return fmt.Errorf("%s: ramping-vus requires stages", label)
		}
		for j, st := range e.Stages {
			if _, err := time.ParseDuration(st.Duration); err != nil {
				return fmt.Errorf("%s: stage[%d] invalid duration: %w", label, j, err)
			}
		}
	default:
		return fmt.Errorf("%s: unknown executor type %q", label, e.Type)
	}
	return nil
}

func (s *Scenario) Expand() (Expanded, error) {
	if s.IsLegacy() {
		return s.expandLegacy()
	}
	return s.expandStateful()
}

func (s *Scenario) expandLegacy() (Expanded, error) {
	dur, err := time.ParseDuration(s.Duration)
	if err != nil {
		return Expanded{}, err
	}
	flow := vu.Flow{
		Scenario: s.Name,
		Steps: []vu.Step{{
			Name:     s.Name,
			Method:   s.Method,
			URLTmpl:  s.URL,
			BodyTmpl: s.Body,
			Headers:  s.Headers,
			Check:    checks.DefaultSpec(),
		}},
	}
	return Expanded{
		Name:     s.Name,
		Flow:     flow,
		Cfg:      executor.VUConfig{Type: "constant-rps", RPS: s.RPS, Duration: dur},
		Identity: s.Identity,
		Weight:   s.Weight,
	}, nil
}

func (s *Scenario) expandStateful() (Expanded, error) {
	flow := vu.Flow{Scenario: s.Name}
	for i := range s.Steps {
		vs, err := s.Steps[i].toVUStep()
		if err != nil {
			return Expanded{}, err
		}
		flow.Steps = append(flow.Steps, vs)
	}
	cfg, err := s.Executor.toVUConfig()
	if err != nil {
		return Expanded{}, err
	}
	return Expanded{Name: s.Name, Flow: flow, Cfg: cfg, Identity: s.Identity, Weight: s.Weight}, nil
}

func (st *Step) toVUStep() (vu.Step, error) {
	out := vu.Step{
		Name:         st.Name,
		Method:       st.Method,
		URLTmpl:      st.URL,
		BodyTmpl:     st.Body,
		Headers:      st.Headers,
		ReadBytesCap: st.ReadBytesCap,
		Once:         st.Once,
		Extract:      toExtractSpecs(st.Extract),
		Check:        toCheckSpec(st.Check),
	}
	if st.Compute != nil {
		var timeout time.Duration
		if st.Compute.Timeout != "" {
			t, err := time.ParseDuration(st.Compute.Timeout)
			if err != nil {
				return vu.Step{}, fmt.Errorf("step %q compute.timeout: %w", st.Name, err)
			}
			timeout = t
		}
		out.Compute = &vu.Compute{
			PrefixTmpl:    st.Compute.Prefix,
			Separator:     st.Compute.Separator,
			Difficulty:    st.Compute.Difficulty,
			DifficultyVar: st.Compute.DifficultyVar,
			MaxIters:      st.Compute.MaxIters,
			Timeout:       timeout,
			Out:           st.Compute.Out,
		}
	}
	return out, nil
}

func toExtractSpecs(in []ExtractSpec) []extract.Spec {
	if len(in) == 0 {
		return nil
	}
	out := make([]extract.Spec, len(in))
	for i, e := range in {
		out[i] = extract.Spec{
			Var:      e.Var,
			Source:   extract.Source(e.From),
			Path:     e.Path,
			Default:  e.Default,
			Required: e.Required,
		}
	}
	return out
}

func toCheckSpec(c *Check) checks.Spec {
	if c == nil {
		return checks.DefaultSpec()
	}
	spec := checks.Spec{
		ExpectStatus:  c.ExpectStatus,
		MaxDurationMs: c.MaxDurationMs,
		MaxTTFBMs:     c.MaxTTFBMs,
		BodyContains:  c.BodyContains,
		Required:      c.Required,
	}
	if len(c.ExpectStatusRange) == 2 {
		spec.ExpectStatusRange = [2]int{c.ExpectStatusRange[0], c.ExpectStatusRange[1]}
	}
	return spec
}

func (e *Executor) toVUConfig() (executor.VUConfig, error) {
	cfg := executor.VUConfig{Type: e.Type, VUs: e.VUs, RPS: e.RPS, StartVUs: e.StartVUs, MaxVUs: e.MaxVUs}
	if e.Duration != "" {
		d, err := time.ParseDuration(e.Duration)
		if err != nil {
			return cfg, err
		}
		cfg.Duration = d
	}
	if e.GracefulStop != "" {
		d, err := time.ParseDuration(e.GracefulStop)
		if err != nil {
			return cfg, err
		}
		cfg.GracefulStop = d
	}
	for _, s := range e.Stages {
		d, err := time.ParseDuration(s.Duration)
		if err != nil {
			return cfg, err
		}
		cfg.Stages = append(cfg.Stages, executor.Stage{Target: s.Target, Duration: d})
	}
	return cfg, nil
}
