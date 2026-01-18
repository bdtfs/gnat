package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestPanicRecovery(t *testing.T) {
	t.Parallel()

	t.Run("normal handler", func(t *testing.T) {
		t.Parallel()

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})

		wrapped := panicRecovery(handler)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("panicking handler", func(t *testing.T) {
		t.Parallel()

		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("test panic")
		})

		wrapped := panicRecovery(handler)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		// Should not panic
		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

func TestLogging(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("logs request", func(t *testing.T) {
		t.Parallel()

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := logging(logger)(handler)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("logs error status", func(t *testing.T) {
		t.Parallel()

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})

		wrapped := logging(logger)(handler)
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestResponseWriter(t *testing.T) {
	t.Parallel()

	t.Run("default status code", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		if rw.statusCode != http.StatusOK {
			t.Errorf("expected default status %d, got %d", http.StatusOK, rw.statusCode)
		}
	})

	t.Run("write header updates status code", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.WriteHeader(http.StatusNotFound)

		if rw.statusCode != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rw.statusCode)
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected recorder status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}
