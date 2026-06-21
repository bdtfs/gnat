package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	httpclient "github.com/bdtfs/gnat/pkg/clients/http"
)

func TestSend_AppliesConfiguredHeaders(t *testing.T) {
	t.Parallel()

	var gotUA, gotAccept, gotCustom string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotCustom = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	headers := map[string]string{
		"User-Agent": "gnat-loadtest/1.0",
		"Accept":     "application/json",
		"X-Custom":   "abc123",
	}

	res := send(context.Background(), httpclient.New(), http.MethodGet, srv.URL, nil, headers)
	if res.Error != nil {
		t.Fatalf("unexpected send error: %v", res.Error)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	if gotUA != "gnat-loadtest/1.0" {
		t.Errorf("User-Agent not applied: got %q", gotUA)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept not applied: got %q", gotAccept)
	}
	if gotCustom != "abc123" {
		t.Errorf("X-Custom not applied: got %q", gotCustom)
	}
}

func TestSend_BodyContentTypeOverridableByHeaders(t *testing.T) {
	t.Parallel()

	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	headers := map[string]string{"Content-Type": "application/json"}

	res := send(context.Background(), httpclient.New(), http.MethodPost, srv.URL, []byte(`{"a":1}`), headers)
	if res.Error != nil {
		t.Fatalf("unexpected send error: %v", res.Error)
	}
	if gotCT != "application/json" {
		t.Errorf("expected configured Content-Type to win, got %q", gotCT)
	}
}
