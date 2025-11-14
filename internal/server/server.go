package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/service"
)

type Server struct {
	addr    string
	service *service.Service
	server  *http.Server
}

func New(addr string, service *service.Service) *Server {
	s := &Server{
		addr:    addr,
		service: service,
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

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
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
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("server shutdown error: %v\n", err)
		}
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
		Body        []byte            `json:"body,omitempty"`
		Headers     map[string]string `json:"headers,omitempty"`
		RPS         int               `json:"rps"`
		Duration    string            `json:"duration"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	duration, err := time.ParseDuration(req.Duration)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid duration format")
		return
	}

	setup, err := s.service.CreateSetup(req.Name, req.Description, req.Method, req.URL, req.Body, req.Headers, req.RPS, duration)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, setup)
}

func (s *Server) handleListSetups(w http.ResponseWriter, r *http.Request) {
	setups := s.service.ListSetups()
	respondJSON(w, http.StatusOK, setups)
}

func (s *Server) handleGetSetup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	setup, err := s.service.GetSetup(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, setup)
}

func (s *Server) handleDeleteSetup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.service.DeleteSetup(id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SetupID string `json:"setup_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := s.service.StartRun(r.Context(), req.SetupID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, run)
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	setupID := r.URL.Query().Get("setup_id")

	var runs []*models.Run
	if setupID != "" {
		runs = s.service.ListRunsBySetup(setupID)
	} else {
		runs = s.service.ListRuns()
	}

	respondJSON(w, http.StatusOK, runs)
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	run, err := s.service.GetRun(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, run)
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.service.CancelRun(id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
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

	respondJSON(w, http.StatusOK, run.Stats)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
