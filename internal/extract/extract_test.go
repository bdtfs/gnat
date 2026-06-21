package extract

import (
	"net/http"
	"testing"
)

func TestJSONPath(t *testing.T) {
	t.Parallel()

	body := []byte(`{"shows":[{"slug":"spongebob"},{"slug":"patrick"}],"data":{"items":[{"id":1},{"id":2},{"id":3}]},"n":42,"pi":3.5,"ok":true,"name":"bob","nil":null}`)

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "array index then key", path: "shows[0].slug", want: "spongebob"},
		{name: "second array element", path: "shows[1].slug", want: "patrick"},
		{name: "nested array index", path: "data.items[2].id", want: "3"},
		{name: "integral number no decimal", path: "n", want: "42"},
		{name: "float number", path: "pi", want: "3.5"},
		{name: "bool true", path: "ok", want: "true"},
		{name: "plain string", path: "name", want: "bob"},
		{name: "null renders empty", path: "nil", want: ""},
		{name: "missing key", path: "missing", wantErr: true},
		{name: "index out of range", path: "shows[9].slug", wantErr: true},
		{name: "key on array", path: "shows.slug", wantErr: true},
		{name: "index on object", path: "data[0]", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := JSONPath(body, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for path %q, got %q", tt.path, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for path %q: %v", tt.path, err)
			}
			if got != tt.want {
				t.Errorf("path %q: got %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestJSONPath_MalformedBody(t *testing.T) {
	t.Parallel()

	_, err := JSONPath([]byte(`{not json`), "a")
	if err == nil {
		t.Fatal("expected error on malformed json")
	}
}

func TestExtract(t *testing.T) {
	t.Parallel()

	in := Input{
		Status: 200,
		Headers: http.Header{
			"X-Request-Id": []string{"req-99"},
			"Empty-Header": []string{""},
		},
		Cookies: []*http.Cookie{
			{Name: "session", Value: "tok-abc"},
			{Name: "blank", Value: ""},
		},
		Body: []byte(`{"shows":[{"slug":"spongebob"}],"n":42,"token":"xyz-7"}`),
	}

	tests := []struct {
		name    string
		spec    Spec
		want    string
		wantErr bool
	}{
		{
			name: "json path",
			spec: Spec{Var: "slug", Source: SourceJSON, Path: "shows[0].slug"},
			want: "spongebob",
		},
		{
			name: "regex capture group",
			spec: Spec{Var: "tok", Source: SourceRegex, Path: `"token":"([^"]+)"`},
			want: "xyz-7",
		},
		{
			name: "regex whole match no group",
			spec: Spec{Var: "tok", Source: SourceRegex, Path: `xyz-\d`},
			want: "xyz-7",
		},
		{
			name: "header",
			spec: Spec{Var: "rid", Source: SourceHeader, Path: "X-Request-Id"},
			want: "req-99",
		},
		{
			name: "cookie",
			spec: Spec{Var: "sess", Source: SourceCookie, Path: "session"},
			want: "tok-abc",
		},
		{
			name: "status ignores path",
			spec: Spec{Var: "code", Source: SourceStatus, Path: "ignored"},
			want: "200",
		},
		{
			name: "missing not required no default returns empty",
			spec: Spec{Var: "x", Source: SourceJSON, Path: "nope"},
			want: "",
		},
		{
			name: "missing with default returns default",
			spec: Spec{Var: "x", Source: SourceJSON, Path: "nope", Default: "fallback"},
			want: "fallback",
		},
		{
			name: "empty header with default returns default",
			spec: Spec{Var: "x", Source: SourceHeader, Path: "Empty-Header", Default: "dflt"},
			want: "dflt",
		},
		{
			name: "empty cookie with default returns default",
			spec: Spec{Var: "x", Source: SourceCookie, Path: "blank", Default: "dflt"},
			want: "dflt",
		},
		{
			name:    "missing required returns error",
			spec:    Spec{Var: "x", Source: SourceJSON, Path: "nope", Required: true},
			wantErr: true,
		},
		{
			name:    "missing header required returns error",
			spec:    Spec{Var: "x", Source: SourceHeader, Path: "No-Such", Required: true},
			wantErr: true,
		},
		{
			name:    "missing cookie required returns error",
			spec:    Spec{Var: "x", Source: SourceCookie, Path: "no-such", Required: true},
			wantErr: true,
		},
		{
			name:    "invalid regex returns error",
			spec:    Spec{Var: "x", Source: SourceRegex, Path: `([`, Required: true},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Extract(tt.spec, in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got value %q", got)
				}
				if _, ok := err.(*Error); !ok {
					t.Fatalf("expected *Error, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtract_MalformedJSONBody(t *testing.T) {
	t.Parallel()

	in := Input{Body: []byte(`{broken`)}
	_, err := Extract(Spec{Var: "x", Source: SourceJSON, Path: "a", Required: true}, in)
	if err == nil {
		t.Fatal("expected error on malformed json body")
	}
	if _, ok := err.(*Error); !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
}

func TestExtractAll(t *testing.T) {
	t.Parallel()

	in := Input{
		Status:  200,
		Headers: http.Header{"X-Trace": []string{"trace-1"}},
		Cookies: []*http.Cookie{{Name: "sid", Value: "cookie-val"}},
		Body:    []byte(`{"shows":[{"slug":"spongebob"}],"n":42,"token":"cap-77"}`),
	}

	specs := []Spec{
		{Var: "slug", Source: SourceJSON, Path: "shows[0].slug"},
		{Var: "cap", Source: SourceRegex, Path: `"token":"([^"]+)"`},
		{Var: "trace", Source: SourceHeader, Path: "X-Trace"},
		{Var: "sid", Source: SourceCookie, Path: "sid"},
		{Var: "code", Source: SourceStatus},
		{Var: "missing", Source: SourceJSON, Path: "absent", Required: true},
	}

	vars, errs := ExtractAll(specs, in)

	wantVars := map[string]string{
		"slug":  "spongebob",
		"cap":   "cap-77",
		"trace": "trace-1",
		"sid":   "cookie-val",
		"code":  "200",
	}
	for k, v := range wantVars {
		if vars[k] != v {
			t.Errorf("var %q: got %q, want %q", k, vars[k], v)
		}
	}
	if _, ok := vars["missing"]; ok {
		t.Errorf("errored var %q should not be in vars map", "missing")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Var != "missing" {
		t.Errorf("expected error for var %q, got %q", "missing", errs[0].Var)
	}
	if errs[0].Source != SourceJSON {
		t.Errorf("expected error source json, got %q", errs[0].Source)
	}
}

func TestError_Error(t *testing.T) {
	t.Parallel()

	e := &Error{Var: "slug", Source: SourceJSON, Path: "a.b", Reason: "value not found"}
	got := e.Error()
	if got == "" {
		t.Fatal("expected non-empty error string")
	}
	for _, sub := range []string{"slug", "json", "a.b", "value not found"} {
		if !contains(got, sub) {
			t.Errorf("error string %q missing %q", got, sub)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
