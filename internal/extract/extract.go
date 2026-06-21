package extract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type Source string

const (
	SourceJSON   Source = "json"
	SourceRegex  Source = "regex"
	SourceHeader Source = "header"
	SourceCookie Source = "cookie"
	SourceStatus Source = "status"
)

type Spec struct {
	Var      string
	Source   Source
	Path     string
	Default  string
	Required bool
}

type Input struct {
	Status  int
	Headers http.Header
	Cookies []*http.Cookie
	Body    []byte
}

type Error struct {
	Var    string
	Source Source
	Path   string
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("extract %q from %s %q: %s", e.Var, e.Source, e.Path, e.Reason)
}

func Extract(spec Spec, in Input) (string, error) {
	value, found, err := resolve(spec, in)
	if err != nil {
		return "", err
	}
	if !found || value == "" {
		if spec.Default != "" {
			return spec.Default, nil
		}
		if spec.Required {
			return "", &Error{Var: spec.Var, Source: spec.Source, Path: spec.Path, Reason: "value not found"}
		}
		return "", nil
	}
	return value, nil
}

func ExtractAll(specs []Spec, in Input) (map[string]string, []Error) {
	vars := make(map[string]string)
	var errs []Error
	for _, spec := range specs {
		value, err := Extract(spec, in)
		if err != nil {
			if ee, ok := err.(*Error); ok {
				errs = append(errs, *ee)
			} else {
				errs = append(errs, Error{Var: spec.Var, Source: spec.Source, Path: spec.Path, Reason: err.Error()})
			}
			continue
		}
		vars[spec.Var] = value
	}
	return vars, errs
}

func resolve(spec Spec, in Input) (string, bool, error) {
	switch spec.Source {
	case SourceJSON:
		value, err := JSONPath(in.Body, spec.Path)
		if err != nil {
			if _, ok := err.(*notFoundError); ok {
				return "", false, nil
			}
			return "", false, &Error{Var: spec.Var, Source: spec.Source, Path: spec.Path, Reason: err.Error()}
		}
		return value, true, nil
	case SourceRegex:
		re, err := regexp.Compile(spec.Path)
		if err != nil {
			return "", false, &Error{Var: spec.Var, Source: spec.Source, Path: spec.Path, Reason: err.Error()}
		}
		m := re.FindStringSubmatch(string(in.Body))
		if m == nil {
			return "", false, nil
		}
		if len(m) > 1 {
			return m[1], true, nil
		}
		return m[0], true, nil
	case SourceHeader:
		if in.Headers == nil {
			return "", false, nil
		}
		v := in.Headers.Get(spec.Path)
		return v, v != "", nil
	case SourceCookie:
		for _, c := range in.Cookies {
			if c != nil && c.Name == spec.Path {
				return c.Value, c.Value != "", nil
			}
		}
		return "", false, nil
	case SourceStatus:
		return strconv.Itoa(in.Status), true, nil
	default:
		return "", false, &Error{Var: spec.Var, Source: spec.Source, Path: spec.Path, Reason: "unknown source"}
	}
}

type notFoundError struct {
	path string
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf("path %q not found", e.path)
}

func JSONPath(body []byte, path string) (string, error) {
	var root interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return "", fmt.Errorf("invalid json: %w", err)
	}
	cur := root
	for _, seg := range splitPath(path) {
		key, idx, hasIdx := parseSegment(seg)
		if key != "" {
			m, ok := cur.(map[string]interface{})
			if !ok {
				return "", &notFoundError{path: path}
			}
			next, ok := m[key]
			if !ok {
				return "", &notFoundError{path: path}
			}
			cur = next
		}
		if hasIdx {
			arr, ok := cur.([]interface{})
			if !ok {
				return "", &notFoundError{path: path}
			}
			if idx < 0 || idx >= len(arr) {
				return "", &notFoundError{path: path}
			}
			cur = arr[idx]
		}
	}
	return render(cur), nil
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

func parseSegment(seg string) (string, int, bool) {
	open := strings.Index(seg, "[")
	if open < 0 {
		return seg, 0, false
	}
	key := seg[:open]
	rest := seg[open:]
	rest = strings.TrimPrefix(rest, "[")
	rest = strings.TrimSuffix(rest, "]")
	idx, err := strconv.Atoi(rest)
	if err != nil {
		return seg, 0, false
	}
	return key, idx, true
}

func render(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
