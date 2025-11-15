package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bdtfs/gnat/internal/converters"
	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/server/dto"
	"github.com/bdtfs/gnat/internal/service"
)

type Server struct {
	addr    string
	service *service.Service
	logger  *slog.Logger
	server  *http.Server
}

func New(addr string, service *service.Service, logger *slog.Logger) *Server {
	s := &Server{
		addr:    addr,
		service: service,
		logger:  logger,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/setups", s.handleCreateSetup)
	mux.HandleFunc("GET /api/setups", s.handleListSetups)
	mux.HandleFunc("GET /api/setups/{id}", s.handleGetSetup)
	mux.HandleFunc("DELETE /api/setups/{id}", s.handleDeleteSetup)

	mux.HandleFunc("POST /api/runs", s.handleStartRun)
	mux.HandleFunc("GET /api/runs", s.handleListRuns)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("POST /api/runs/{id}/cancel", s.handleCancelRun)
	mux.HandleFunc("GET /api/runs/{id}/stats", s.handleGetRunStats)

	handler := panicRecovery(logging(logger)(mux))

	s.server = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) handleCreateSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Method      string            `json:"method"`
		URL         string            `json:"url"`
		Body        []byte            `json:"body"`
		Headers     map[string]string `json:"headers"`
		RPS         int               `json:"rps"`
		Duration    string            `json:"duration"`
	}

	if json.NewDecoder(r.Body).Decode(&req) != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	dur, err := time.ParseDuration(req.Duration)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid duration")
		return
	}

	m, err := s.service.CreateSetup(req.Name, req.Description, req.Method, req.URL, req.Body, req.Headers, req.RPS, dur)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, converters.SetupToDTO(m))
}

func (s *Server) handleListSetups(w http.ResponseWriter, _ *http.Request) {
	setups := s.service.ListSetups()

	out := make([]*dto.Setup, len(setups))
	for i, m := range setups {
		out[i] = converters.SetupToDTO(m)
	}

	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetSetup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	m, err := s.service.GetSetup(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, converters.SetupToDTO(m))
}

func (s *Server) handleDeleteSetup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if s.service.DeleteSetup(id) != nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SetupID string `json:"setup_id"`
	}

	if json.NewDecoder(r.Body).Decode(&req) != nil {
		respondError(w, http.StatusBadRequest, "invalid request")
		return
	}

	m, err := s.service.StartRun(r.Context(), req.SetupID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	out := converters.RunToDTO(m)
	respondJSON(w, http.StatusCreated, out)
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	setupID := r.URL.Query().Get("setup_id")

	var modelsRuns []*models.Run
	switch setupID {
	case "":
		modelsRuns = s.service.ListRuns()
	default:
		modelsRuns = s.service.ListRunsBySetup(setupID)
	}

	out := make([]*dto.Run, len(modelsRuns))
	for i, m := range modelsRuns {
		out[i] = converters.RunToDTO(m)
	}

	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	m, err := s.service.GetRun(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, converters.RunToDTO(m))
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if s.service.CancelRun(id) != nil {
		respondError(w, http.StatusBadRequest, "cannot cancel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetRunStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	run, err := s.service.GetRun(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, converters.RunToDTO(run).Stats)
}

func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}
