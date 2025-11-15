package di

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/bdtfs/gnat/internal/config"
	"github.com/bdtfs/gnat/internal/runner"
	"github.com/bdtfs/gnat/internal/server"
	"github.com/bdtfs/gnat/internal/service"
	repository "github.com/bdtfs/gnat/internal/storage/memory"
	httpclient "github.com/bdtfs/gnat/pkg/clients/http"
)

type Container struct {
	ctx context.Context
	cfg config.Config

	httpClient     *http.Client
	httpClientOnce sync.Once

	repo     *repository.Repository
	repoOnce sync.Once

	collector     *runner.Collector
	collectorOnce sync.Once

	runner     *runner.Runner
	runnerOnce sync.Once

	service     *service.Service
	serviceOnce sync.Once

	server     *server.Server
	serverOnce sync.Once

	logger     *slog.Logger
	loggerOnce sync.Once
}

func New(ctx context.Context) *Container {
	return &Container{
		ctx: ctx,
		cfg: config.MustLoad(),
	}
}

func (c *Container) GetHTTPClient() *http.Client {
	c.httpClientOnce.Do(func() {
		cfg := &httpclient.Config{
			MaxIdleConns:        c.cfg.HTTPClientConfig.MaxIdleConns,
			MaxIdleConnsPerHost: c.cfg.HTTPClientConfig.MaxIdleConnsPerHost,
			IdleConnTimeout:     c.cfg.HTTPClientConfig.IdleConnTimeout,
			DisableCompression:  c.cfg.HTTPClientConfig.DisableCompression,
			DialTimeout:         c.cfg.HTTPClientConfig.DialTimeout,
			KeepAlive:           c.cfg.HTTPClientConfig.KeepAlive,
			TLSHandshakeTimeout: c.cfg.HTTPClientConfig.TLSHandshakeTimeout,
			ExpectTimeout:       c.cfg.HTTPClientConfig.ExpectTimeout,
			RequestTimeout:      c.cfg.HTTPClientConfig.RequestTimeout,
		}
		c.httpClient = httpclient.WithConfig(cfg)
	})
	return c.httpClient
}

func (c *Container) GetLogger() *slog.Logger {
	c.loggerOnce.Do(func() {
		opts := &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
		handler := slog.NewJSONHandler(os.Stdout, opts)
		c.logger = slog.New(handler)
	})
	return c.logger
}

func (c *Container) GetRepository() *repository.Repository {
	c.repoOnce.Do(func() {
		c.repo = repository.New()
	})
	return c.repo
}

func (c *Container) GetCollector() *runner.Collector {
	c.collectorOnce.Do(func() {
		c.collector = runner.NewCollector()
	})
	return c.collector
}

func (c *Container) GetRunner() *runner.Runner {
	c.runnerOnce.Do(func() {
		c.runner = runner.New(c.GetRepository(), c.GetLogger(), c.GetCollector())
	})
	return c.runner
}

func (c *Container) GetService() *service.Service {
	c.serviceOnce.Do(func() {
		c.service = service.New(c.GetRepository(), c.GetRunner())
	})
	return c.service
}

func (c *Container) GetServer() *server.Server {
	c.serverOnce.Do(func() {
		addr := net.JoinHostPort("", strconv.Itoa(c.GetConfig().Application.Port))
		c.server = server.New(addr, c.GetService(), c.GetLogger())
	})
	return c.server
}

func (c *Container) GetConfig() config.Config {
	return c.cfg
}

func (c *Container) GetContext() context.Context {
	return c.ctx
}

func (c *Container) Shutdown() {
	if c.runner != nil {
		c.runner.Shutdown()
	}
}
