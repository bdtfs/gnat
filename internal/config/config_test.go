package config

import (
	"os"
	"testing"
	"time"
)

func TestMustLoad_Defaults(t *testing.T) {
	// Clear any env vars that might affect the test
	envVars := []string{
		"APPLICATION_PORT",
		"HTTP_MAX_IDLE_CONNS",
		"HTTP_MAX_IDLE_CONNS_PER_HOST",
		"HTTP_IDLE_CONN_TIMEOUT",
		"HTTP_DISABLE_COMPRESSION",
		"HTTP_DIAL_TIMEOUT",
		"HTTP_KEEPALIVE",
		"HTTP_TLS_HANDSHAKE_TIMEOUT",
		"HTTP_EXPECT_TIMEOUT",
		"HTTP_REQUEST_TIMEOUT",
	}

	for _, key := range envVars {
		os.Unsetenv(key)
	}

	cfg := MustLoad()

	if cfg.Application == nil {
		t.Fatal("expected Application to be non-nil")
	}
	if cfg.Application.Port != 8778 {
		t.Errorf("expected default Port 8778, got %d", cfg.Application.Port)
	}

	if cfg.HTTPClientConfig == nil {
		t.Fatal("expected HTTPClientConfig to be non-nil")
	}
	if cfg.HTTPClientConfig.MaxIdleConns != 10000 {
		t.Errorf("expected default MaxIdleConns 10000, got %d", cfg.HTTPClientConfig.MaxIdleConns)
	}
	if cfg.HTTPClientConfig.MaxIdleConnsPerHost != 10000 {
		t.Errorf("expected default MaxIdleConnsPerHost 10000, got %d", cfg.HTTPClientConfig.MaxIdleConnsPerHost)
	}
	if cfg.HTTPClientConfig.IdleConnTimeout != 90*time.Second {
		t.Errorf("expected default IdleConnTimeout 90s, got %v", cfg.HTTPClientConfig.IdleConnTimeout)
	}
	if cfg.HTTPClientConfig.DisableCompression != false {
		t.Errorf("expected default DisableCompression false, got %v", cfg.HTTPClientConfig.DisableCompression)
	}
	if cfg.HTTPClientConfig.DialTimeout != 5*time.Second {
		t.Errorf("expected default DialTimeout 5s, got %v", cfg.HTTPClientConfig.DialTimeout)
	}
	if cfg.HTTPClientConfig.KeepAlive != 30*time.Second {
		t.Errorf("expected default KeepAlive 30s, got %v", cfg.HTTPClientConfig.KeepAlive)
	}
	if cfg.HTTPClientConfig.TLSHandshakeTimeout != 5*time.Second {
		t.Errorf("expected default TLSHandshakeTimeout 5s, got %v", cfg.HTTPClientConfig.TLSHandshakeTimeout)
	}
	if cfg.HTTPClientConfig.ExpectTimeout != 1*time.Second {
		t.Errorf("expected default ExpectTimeout 1s, got %v", cfg.HTTPClientConfig.ExpectTimeout)
	}
	if cfg.HTTPClientConfig.RequestTimeout != 10*time.Second {
		t.Errorf("expected default RequestTimeout 10s, got %v", cfg.HTTPClientConfig.RequestTimeout)
	}
}

func TestMustLoad_WithEnvVars(t *testing.T) {
	// Set custom env vars
	os.Setenv("APPLICATION_PORT", "9000")
	os.Setenv("HTTP_MAX_IDLE_CONNS", "5000")
	os.Setenv("HTTP_REQUEST_TIMEOUT", "30s")
	defer func() {
		os.Unsetenv("APPLICATION_PORT")
		os.Unsetenv("HTTP_MAX_IDLE_CONNS")
		os.Unsetenv("HTTP_REQUEST_TIMEOUT")
	}()

	cfg := MustLoad()

	if cfg.Application.Port != 9000 {
		t.Errorf("expected Port 9000, got %d", cfg.Application.Port)
	}
	if cfg.HTTPClientConfig.MaxIdleConns != 5000 {
		t.Errorf("expected MaxIdleConns 5000, got %d", cfg.HTTPClientConfig.MaxIdleConns)
	}
	if cfg.HTTPClientConfig.RequestTimeout != 30*time.Second {
		t.Errorf("expected RequestTimeout 30s, got %v", cfg.HTTPClientConfig.RequestTimeout)
	}
}

func TestGetEnv_Int(t *testing.T) {
	tests := []struct {
		name       string
		envKey     string
		envValue   string
		defaultVal int
		expected   int
	}{
		{
			name:       "env not set returns default",
			envKey:     "TEST_INT_NOT_SET",
			envValue:   "",
			defaultVal: 42,
			expected:   42,
		},
		{
			name:       "valid int value",
			envKey:     "TEST_INT_VALID",
			envValue:   "100",
			defaultVal: 42,
			expected:   100,
		},
		{
			name:       "invalid int returns default",
			envKey:     "TEST_INT_INVALID",
			envValue:   "not-a-number",
			defaultVal: 42,
			expected:   42,
		},
		{
			name:       "zero value",
			envKey:     "TEST_INT_ZERO",
			envValue:   "0",
			defaultVal: 42,
			expected:   0,
		},
		{
			name:       "negative value",
			envKey:     "TEST_INT_NEGATIVE",
			envValue:   "-10",
			defaultVal: 42,
			expected:   -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}

			result := getEnv(tt.envKey, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGetEnv_Bool(t *testing.T) {
	tests := []struct {
		name       string
		envKey     string
		envValue   string
		defaultVal bool
		expected   bool
	}{
		{
			name:       "env not set returns default true",
			envKey:     "TEST_BOOL_NOT_SET",
			envValue:   "",
			defaultVal: true,
			expected:   true,
		},
		{
			name:       "env not set returns default false",
			envKey:     "TEST_BOOL_NOT_SET2",
			envValue:   "",
			defaultVal: false,
			expected:   false,
		},
		{
			name:       "true value",
			envKey:     "TEST_BOOL_TRUE",
			envValue:   "true",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "false value",
			envKey:     "TEST_BOOL_FALSE",
			envValue:   "false",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "1 as true",
			envKey:     "TEST_BOOL_ONE",
			envValue:   "1",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "0 as false",
			envKey:     "TEST_BOOL_ZERO",
			envValue:   "0",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "invalid returns default",
			envKey:     "TEST_BOOL_INVALID",
			envValue:   "yes",
			defaultVal: false,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}

			result := getEnv(tt.envKey, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetEnv_Duration(t *testing.T) {
	tests := []struct {
		name       string
		envKey     string
		envValue   string
		defaultVal time.Duration
		expected   time.Duration
	}{
		{
			name:       "env not set returns default",
			envKey:     "TEST_DUR_NOT_SET",
			envValue:   "",
			defaultVal: 5 * time.Second,
			expected:   5 * time.Second,
		},
		{
			name:       "seconds",
			envKey:     "TEST_DUR_SECONDS",
			envValue:   "30s",
			defaultVal: 5 * time.Second,
			expected:   30 * time.Second,
		},
		{
			name:       "minutes",
			envKey:     "TEST_DUR_MINUTES",
			envValue:   "2m",
			defaultVal: 5 * time.Second,
			expected:   2 * time.Minute,
		},
		{
			name:       "milliseconds",
			envKey:     "TEST_DUR_MS",
			envValue:   "500ms",
			defaultVal: 5 * time.Second,
			expected:   500 * time.Millisecond,
		},
		{
			name:       "invalid returns default",
			envKey:     "TEST_DUR_INVALID",
			envValue:   "not-a-duration",
			defaultVal: 5 * time.Second,
			expected:   5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}

			result := getEnv(tt.envKey, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParse_Int(t *testing.T) {
	tests := []struct {
		name    string
		val     string
		wantErr bool
		want    int
	}{
		{"valid positive", "42", false, 42},
		{"valid negative", "-10", false, -10},
		{"valid zero", "0", false, 0},
		{"invalid", "abc", true, 0},
		{"empty", "", true, 0},
		{"float", "3.14", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parse(tt.val, 0)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.want {
					t.Errorf("expected %d, got %d", tt.want, result)
				}
			}
		})
	}
}

func TestParse_Bool(t *testing.T) {
	tests := []struct {
		name    string
		val     string
		wantErr bool
		want    bool
	}{
		{"true", "true", false, true},
		{"false", "false", false, false},
		{"1", "1", false, true},
		{"0", "0", false, false},
		{"True", "True", false, true},
		{"FALSE", "FALSE", false, false},
		{"invalid", "yes", true, false},
		{"empty", "", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parse(tt.val, false)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.want {
					t.Errorf("expected %v, got %v", tt.want, result)
				}
			}
		})
	}
}

func TestParse_Duration(t *testing.T) {
	tests := []struct {
		name    string
		val     string
		wantErr bool
		want    time.Duration
	}{
		{"seconds", "30s", false, 30 * time.Second},
		{"minutes", "5m", false, 5 * time.Minute},
		{"hours", "1h", false, time.Hour},
		{"milliseconds", "100ms", false, 100 * time.Millisecond},
		{"complex", "1h30m", false, time.Hour + 30*time.Minute},
		{"invalid", "invalid", true, 0},
		{"empty", "", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parse(tt.val, time.Duration(0))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.want {
					t.Errorf("expected %v, got %v", tt.want, result)
				}
			}
		})
	}
}
