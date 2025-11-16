package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bdtfs/gnat/internal/web"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := getEnvInt("FRONTEND_PORT", 3000)
	apiURL := getEnv("API_URL", "http://localhost:8778")

	mux := http.NewServeMux()
	handler := web.NewHandler(apiURL, logger)
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.Info("frontend starting", "port", port, "api_url", apiURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("frontend error", "error", err)
			os.Exit(1)
		}
	}()

	printWelcome(port, apiURL)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down frontend")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("frontend forced shutdown", "error", err)
	}

	logger.Info("frontend stopped")
}

func printWelcome(port int, apiURL string) {
	fmt.Printf(`
╔═══════════════════════════════════════════╗
║         🦟 gnat Frontend                  ║
║    Load Testing Web Interface             ║
╚═══════════════════════════════════════════╝

Web UI:     http://localhost:%d
Backend:    %s

Press Ctrl+C to stop
`, port, apiURL)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		fmt.Sscanf(value, "%d", &i)
		return i
	}
	return fallback
}
