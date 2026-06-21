package vu

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bdtfs/gnat/internal/checks"
	"github.com/bdtfs/gnat/internal/extract"
	"github.com/bdtfs/gnat/internal/metrics"
	"github.com/bdtfs/gnat/internal/pow"
	httpclient "github.com/bdtfs/gnat/pkg/clients/http"
)

type Compute struct {
	PrefixTmpl    string
	Separator     string
	Difficulty    int
	DifficultyVar string
	MaxIters      uint64
	Timeout       time.Duration
	Out           string
}

type Step struct {
	Name         string
	Method       string
	URLTmpl      string
	BodyTmpl     string
	Headers      map[string]string
	ReadBytesCap int64
	Compute      *Compute
	Extract      []extract.Spec
	Check        checks.Spec
	Once         bool
}

type Flow struct {
	Scenario string
	Steps    []Step
}

type StepResult struct {
	Scenario    string
	Step        string
	Status      int
	DurationMs  float64
	TTFBMs      float64
	BytesRead   int64
	CheckPassed bool
	Aborted     bool
	Err         error
}

type IterResult struct {
	Completed bool
	Steps     []StepResult
}

type Factory struct {
	transport http.RoundTripper
	timeout   time.Duration
	globals   map[string]string
}

func NewFactory(cfg *httpclient.Config, globals map[string]string) *Factory {
	base := httpclient.WithConfig(cfg)
	return &Factory{transport: base.Transport, timeout: base.Timeout, globals: globals}
}

type VU struct {
	Client   *http.Client
	Jar      http.CookieJar
	Vars     map[string]string
	Identity map[string]string
	ranOnce  map[string]struct{}
}

func (f *Factory) New(identity map[string]string) (*VU, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	vars := make(map[string]string, len(f.globals))
	for k, v := range f.globals {
		vars[k] = v
	}
	return &VU{
		Client:   &http.Client{Transport: f.transport, Timeout: f.timeout, Jar: jar},
		Jar:      jar,
		Vars:     vars,
		Identity: identity,
		ranOnce:  make(map[string]struct{}),
	}, nil
}

func (v *VU) Do(ctx context.Context, method, url, body string, headers map[string]string, readCap int64) (int, http.Header, []*http.Cookie, []byte, float64, float64, error) {
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return 0, nil, nil, nil, 0, 0, err
	}
	for k, val := range v.Identity {
		req.Header.Set(k, val)
	}
	for k, val := range headers {
		req.Header.Set(k, val)
	}

	var ttfb time.Duration
	start := time.Now()
	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() { ttfb = time.Since(start) },
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	resp, err := v.Client.Do(req)
	if err != nil {
		return 0, nil, nil, nil, 0, msSince(start), err
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if readCap > 0 {
		reader = io.LimitReader(resp.Body, readCap)
	}
	bodyBytes, readErr := io.ReadAll(reader)
	_, _ = io.Copy(io.Discard, resp.Body)
	dur := msSince(start)
	if readErr != nil {
		return resp.StatusCode, resp.Header, resp.Cookies(), bodyBytes, ms(ttfb), dur, readErr
	}
	return resp.StatusCode, resp.Header, resp.Cookies(), bodyBytes, ms(ttfb), dur, nil
}

func (v *VU) RunIteration(ctx context.Context, f Flow, sink metrics.Sink) IterResult {
	res := IterResult{Completed: true}
	for _, st := range f.Steps {
		if st.Once {
			if _, done := v.ranOnce[st.Name]; done {
				continue
			}
			v.ranOnce[st.Name] = struct{}{}
		}
		var sr StepResult
		if st.Compute != nil {
			sr = v.runCompute(ctx, f.Scenario, st)
		} else {
			sr = v.runHTTP(ctx, f.Scenario, st)
		}
		res.Steps = append(res.Steps, sr)
		sink.Record(metrics.Sample{
			Scenario:    f.Scenario,
			Step:        st.Name,
			Status:      sr.Status,
			DurationMs:  sr.DurationMs,
			TTFBMs:      sr.TTFBMs,
			BytesRead:   sr.BytesRead,
			CheckPassed: sr.CheckPassed,
			Err:         sr.Err,
		})
		if sr.Aborted {
			res.Completed = false
			break
		}
	}
	return res
}

func (v *VU) runHTTP(ctx context.Context, scenario string, st Step) StepResult {
	sr := StepResult{Scenario: scenario, Step: st.Name}
	url := Interpolate(st.URLTmpl, v.Vars)
	body := Interpolate(st.BodyTmpl, v.Vars)
	var hdrs map[string]string
	if len(st.Headers) > 0 {
		hdrs = make(map[string]string, len(st.Headers))
		for k, val := range st.Headers {
			hdrs[k] = Interpolate(val, v.Vars)
		}
	}
	status, header, cookies, respBody, ttfb, dur, err := v.Do(ctx, st.Method, url, body, hdrs, st.ReadBytesCap)
	sr.Status = status
	sr.TTFBMs = ttfb
	sr.DurationMs = dur
	sr.BytesRead = int64(len(respBody))
	if err != nil {
		sr.Err = err
		sr.CheckPassed = false
		if st.Check.Required {
			sr.Aborted = true
		}
		return sr
	}
	vars, errs := extract.ExtractAll(st.Extract, extract.Input{Status: status, Headers: header, Cookies: cookies, Body: respBody})
	for k, val := range vars {
		v.Vars[k] = val
	}
	oc := checks.Evaluate(st.Check, status, dur, ttfb, respBody)
	sr.CheckPassed = oc.Passed && len(errs) == 0
	if oc.AbortVU || len(errs) > 0 {
		sr.Aborted = true
	}
	return sr
}

func (v *VU) runCompute(ctx context.Context, scenario string, st Step) StepResult {
	sr := StepResult{Scenario: scenario, Step: st.Name}
	start := time.Now()
	prefix := Interpolate(st.Compute.PrefixTmpl, v.Vars)
	diff := st.Compute.Difficulty
	if st.Compute.DifficultyVar != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(Interpolate(st.Compute.DifficultyVar, v.Vars))); err == nil {
			diff = n
		}
	}
	sol, err := pow.Solve(ctx, pow.Challenge{
		Prefix:     prefix,
		Separator:  st.Compute.Separator,
		Difficulty: diff,
		MaxIters:   st.Compute.MaxIters,
		Timeout:    st.Compute.Timeout,
	})
	sr.DurationMs = msSince(start)
	if err != nil {
		sr.Err = err
		sr.CheckPassed = false
		sr.Aborted = true
		return sr
	}
	v.Vars[st.Compute.Out] = sol.Nonce
	sr.CheckPassed = true
	return sr
}

var varPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

func Interpolate(tmpl string, vars map[string]string) string {
	if !strings.Contains(tmpl, "{{") {
		return tmpl
	}
	return varPattern.ReplaceAllStringFunc(tmpl, func(m string) string {
		name := varPattern.FindStringSubmatch(m)[1]
		if val, ok := vars[name]; ok {
			return val
		}
		return m
	})
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func msSince(t time.Time) float64 {
	return ms(time.Since(t))
}
