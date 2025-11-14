package httpclient

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

type Config struct {
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

func DefaultConfig() *Config {
	return &Config{
		MaxIdleConns:        10000,
		MaxIdleConnsPerHost: 10000,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		DialTimeout:         5 * time.Second,
		KeepAlive:           30 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		ExpectTimeout:       1 * time.Second,
		RequestTimeout:      10 * time.Second,
	}
}

func New() *http.Client {
	return WithConfig(DefaultConfig())
}

func WithConfig(cfg *Config) *http.Client {
	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		DisableCompression:    cfg.DisableCompression,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ExpectContinueTimeout: cfg.ExpectTimeout,
		DialContext: (&net.Dialer{
			Timeout:   cfg.DialTimeout,
			KeepAlive: cfg.KeepAlive,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	}

	return &http.Client{
		Timeout:   cfg.RequestTimeout,
		Transport: t,
	}
}
