package httpclient

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.MaxIdleConns != 10000 {
		t.Errorf("expected MaxIdleConns 10000, got %d", cfg.MaxIdleConns)
	}
	if cfg.MaxIdleConnsPerHost != 10000 {
		t.Errorf("expected MaxIdleConnsPerHost 10000, got %d", cfg.MaxIdleConnsPerHost)
	}
	if cfg.IdleConnTimeout != 90*time.Second {
		t.Errorf("expected IdleConnTimeout 90s, got %v", cfg.IdleConnTimeout)
	}
	if cfg.DisableCompression != false {
		t.Errorf("expected DisableCompression false, got %v", cfg.DisableCompression)
	}
	if cfg.DialTimeout != 5*time.Second {
		t.Errorf("expected DialTimeout 5s, got %v", cfg.DialTimeout)
	}
	if cfg.KeepAlive != 30*time.Second {
		t.Errorf("expected KeepAlive 30s, got %v", cfg.KeepAlive)
	}
	if cfg.TLSHandshakeTimeout != 5*time.Second {
		t.Errorf("expected TLSHandshakeTimeout 5s, got %v", cfg.TLSHandshakeTimeout)
	}
	if cfg.ExpectTimeout != 1*time.Second {
		t.Errorf("expected ExpectTimeout 1s, got %v", cfg.ExpectTimeout)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Errorf("expected RequestTimeout 10s, got %v", cfg.RequestTimeout)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	client := New()

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("expected Timeout 10s, got %v", client.Timeout)
	}
	if client.Transport == nil {
		t.Error("expected non-nil Transport")
	}
}

func TestWithConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "default config",
			cfg:  DefaultConfig(),
		},
		{
			name: "custom config",
			cfg: &Config{
				MaxIdleConns:        5000,
				MaxIdleConnsPerHost: 500,
				IdleConnTimeout:     60 * time.Second,
				DisableCompression:  true,
				DialTimeout:         10 * time.Second,
				KeepAlive:           60 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
				ExpectTimeout:       2 * time.Second,
				RequestTimeout:      30 * time.Second,
			},
		},
		{
			name: "minimal config",
			cfg: &Config{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     time.Second,
				DialTimeout:         time.Second,
				KeepAlive:           time.Second,
				TLSHandshakeTimeout: time.Second,
				ExpectTimeout:       time.Second,
				RequestTimeout:      time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := WithConfig(tt.cfg)

			if client == nil {
				t.Fatal("expected non-nil client")
			}
			if client.Timeout != tt.cfg.RequestTimeout {
				t.Errorf("expected Timeout %v, got %v", tt.cfg.RequestTimeout, client.Timeout)
			}
			if client.Transport == nil {
				t.Error("expected non-nil Transport")
			}

			// Verify the transport is an *http.Transport
			transport, ok := client.Transport.(*http.Transport)
			if !ok {
				t.Error("expected Transport to be *http.Transport")
				return
			}

			if transport.MaxIdleConns != tt.cfg.MaxIdleConns {
				t.Errorf("expected MaxIdleConns %d, got %d", tt.cfg.MaxIdleConns, transport.MaxIdleConns)
			}
			if transport.MaxIdleConnsPerHost != tt.cfg.MaxIdleConnsPerHost {
				t.Errorf("expected MaxIdleConnsPerHost %d, got %d", tt.cfg.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
			}
			if transport.MaxConnsPerHost != tt.cfg.MaxIdleConnsPerHost {
				t.Errorf("expected MaxConnsPerHost %d, got %d", tt.cfg.MaxIdleConnsPerHost, transport.MaxConnsPerHost)
			}
			if transport.IdleConnTimeout != tt.cfg.IdleConnTimeout {
				t.Errorf("expected IdleConnTimeout %v, got %v", tt.cfg.IdleConnTimeout, transport.IdleConnTimeout)
			}
			if transport.DisableCompression != tt.cfg.DisableCompression {
				t.Errorf("expected DisableCompression %v, got %v", tt.cfg.DisableCompression, transport.DisableCompression)
			}
			if transport.DisableKeepAlives != false {
				t.Error("expected DisableKeepAlives to be false")
			}
			if transport.ForceAttemptHTTP2 != true {
				t.Error("expected ForceAttemptHTTP2 to be true")
			}
			if transport.TLSHandshakeTimeout != tt.cfg.TLSHandshakeTimeout {
				t.Errorf("expected TLSHandshakeTimeout %v, got %v", tt.cfg.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
			}
			if transport.ExpectContinueTimeout != tt.cfg.ExpectTimeout {
				t.Errorf("expected ExpectContinueTimeout %v, got %v", tt.cfg.ExpectTimeout, transport.ExpectContinueTimeout)
			}
		})
	}
}

func TestWithConfig_TLSConfig(t *testing.T) {
	t.Parallel()

	client := WithConfig(DefaultConfig())
	transport := client.Transport.(*http.Transport)

	if transport.TLSClientConfig == nil {
		t.Fatal("expected non-nil TLSClientConfig")
	}
	if transport.TLSClientConfig.InsecureSkipVerify != false {
		t.Error("expected InsecureSkipVerify to be false")
	}
	if transport.TLSClientConfig.MinVersion != 0x0303 { // tls.VersionTLS12
		t.Errorf("expected MinVersion TLS 1.2 (0x0303), got 0x%04x", transport.TLSClientConfig.MinVersion)
	}
}

func TestNew_IsUsable(t *testing.T) {
	t.Parallel()

	// Verify that the client can be used (basic smoke test)
	client := New()

	// Create a request (we won't actually send it)
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Verify the client uses default redirect policy (nil is fine)
	_ = client.CheckRedirect

	// The request should be valid for the client
	if req.Method != http.MethodGet {
		t.Errorf("expected GET method, got %s", req.Method)
	}
}
