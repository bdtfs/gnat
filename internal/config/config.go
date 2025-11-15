package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Application      *Application
	HTTPClientConfig *HTTPClientConfig
}

type Application struct {
	Port int
}

type HTTPClientConfig struct {
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
	DisableCompression  bool
	DialTimeout         time.Duration
	KeepAlive           time.Duration
	TLSHandshakeTimeout time.Duration
	ExpectTimeout       time.Duration
	RequestTimeout      time.Duration
}

func MustLoad() Config {
	return Config{
		Application: &Application{
			Port: getEnv("APPLICATION_PORT", 8778),
		},
		HTTPClientConfig: &HTTPClientConfig{
			MaxIdleConns:        getEnv("HTTP_MAX_IDLE_CONNS", 10000),
			MaxIdleConnsPerHost: getEnv("HTTP_MAX_IDLE_CONNS_PER_HOST", 10000),
			IdleConnTimeout:     getEnv("HTTP_IDLE_CONN_TIMEOUT", 90*time.Second),
			DisableCompression:  getEnv("HTTP_DISABLE_COMPRESSION", false),
			DialTimeout:         getEnv("HTTP_DIAL_TIMEOUT", 5*time.Second),
			KeepAlive:           getEnv("HTTP_KEEPALIVE", 30*time.Second),
			TLSHandshakeTimeout: getEnv("HTTP_TLS_HANDSHAKE_TIMEOUT", 5*time.Second),
			ExpectTimeout:       getEnv("HTTP_EXPECT_TIMEOUT", 1*time.Second),
			RequestTimeout:      getEnv("HTTP_REQUEST_TIMEOUT", 10*time.Second),
		},
	}
}

func mustGetEnv[T int | bool | time.Duration](key string) T {
	val := os.Getenv(key)
	if val == "" {
		panic("missing required environment variable: " + key)
	}

	var zero T
	result, err := parse[T](val, zero)
	if err != nil {
		panic("failed to parse environment variable " + key + ": " + err.Error())
	}

	return result
}

func getEnv[T int | bool | time.Duration](key string, defaultVal T) T {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}

	result, err := parse[T](val, defaultVal)
	if err != nil {
		return defaultVal
	}

	return result
}

func parse[T int | bool | time.Duration](val string, defaultVal T) (T, error) {
	var result any
	var err error

	switch any(defaultVal).(type) {
	case int:
		result, err = strconv.Atoi(val)
	case bool:
		result, err = strconv.ParseBool(val)
	case time.Duration:
		result, err = time.ParseDuration(val)
	}

	if err != nil {
		var zero T
		return zero, err
	}

	return result.(T), nil
}
