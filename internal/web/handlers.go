package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/* static/*
var content embed.FS

type Handler struct {
	apiBase string
	tmpl    *template.Template
	logger  *slog.Logger
}

func NewHandler(apiBase string, logger *slog.Logger) *Handler {
	funcMap := template.FuncMap{
		"formatTime":     formatTime,
		"formatDuration": formatDuration,
		"formatFloat":    formatFloat,
		"json":           toJSON,
		"mul":            func(a, b float64) float64 { return a * b },
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(content, "templates/*.html"))
	return &Handler{
		apiBase: apiBase,
		tmpl:    tmpl,
		logger:  logger,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.index)
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /static/{file}", h.static)
	mux.HandleFunc("GET /setups", h.listSetups)
	mux.HandleFunc("POST /setups", h.createSetup)
	mux.HandleFunc("POST /runs", h.createRun)
	mux.HandleFunc("GET /runs", h.listRuns)
	mux.HandleFunc("GET /runs/active", h.listActiveRuns)
	mux.HandleFunc("GET /runs/{id}", h.getRunStats)
	mux.HandleFunc("POST /runs/{id}/cancel", h.cancelRun)
	mux.HandleFunc("DELETE /setups/{id}", h.deleteSetup)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	h.tmpl.ExecuteTemplate(w, "index.html", nil)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) static(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")

	if strings.HasSuffix(file, ".css") {
		w.Header().Set("Content-Type", "text/css")
	} else if strings.HasSuffix(file, ".js") {
		w.Header().Set("Content-Type", "application/javascript")
	}

	data, err := content.ReadFile("static/" + file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Write(data)
}

func (h *Handler) listSetups(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(h.apiBase + "/api/setups")
	if err != nil {
		h.logger.Error("failed to fetch setups", "error", err)
		http.Error(w, "Failed to fetch setups", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var setups []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&setups); err != nil {
		h.logger.Error("failed to decode setups", "error", err)
		http.Error(w, "Failed to decode setups", http.StatusInternalServerError)
		return
	}

	h.tmpl.ExecuteTemplate(w, "setups-list.html", setups)
}

func (h *Handler) createSetup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	payload := map[string]interface{}{
		"name":        r.FormValue("name"),
		"description": r.FormValue("description"),
		"method":      r.FormValue("method"),
		"url":         r.FormValue("url"),
		"rps":         parseIntOrDefault(r.FormValue("rps"), 100),
		"duration":    r.FormValue("duration"),
		"headers":     map[string]string{},
		"body":        "",
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(h.apiBase+"/api/setups", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		h.logger.Error("failed to create setup", "error", err)
		http.Error(w, "Failed to create setup", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		h.logger.Error("setup creation failed", "status", resp.StatusCode, "body", string(body))
		http.Error(w, "Setup creation failed", resp.StatusCode)
		return
	}

	w.Header().Set("HX-Trigger", "setupCreated")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) createRun(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	payload := map[string]string{
		"setup_id": r.FormValue("setup_id"),
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(h.apiBase+"/api/runs", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		h.logger.Error("failed to create run", "error", err)
		http.Error(w, "Failed to create run", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		h.logger.Error("run creation failed", "status", resp.StatusCode, "body", string(body))
		http.Error(w, "Run creation failed", resp.StatusCode)
		return
	}

	w.Header().Set("HX-Trigger", "runCreated")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(h.apiBase + "/api/runs")
	if err != nil {
		h.logger.Error("failed to fetch runs", "error", err)
		http.Error(w, "Failed to fetch runs", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var runs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		h.logger.Error("failed to decode runs", "error", err)
		http.Error(w, "Failed to decode runs", http.StatusInternalServerError)
		return
	}

	h.tmpl.ExecuteTemplate(w, "all-runs-list.html", runs)
}

func (h *Handler) listActiveRuns(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(h.apiBase + "/api/runs")
	if err != nil {
		h.logger.Error("failed to fetch runs", "error", err)
		http.Error(w, "Failed to fetch runs", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var allRuns []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&allRuns); err != nil {
		h.logger.Error("failed to decode runs", "error", err)
		http.Error(w, "Failed to decode runs", http.StatusInternalServerError)
		return
	}

	var activeRuns []map[string]interface{}
	for _, run := range allRuns {
		status, ok := run["status"].(string)
		if ok && (status == "pending" || status == "running") {
			activeRuns = append(activeRuns, run)
		}
	}

	h.tmpl.ExecuteTemplate(w, "runs-list.html", activeRuns)
}

func (h *Handler) getRunStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	resp, err := http.Get(h.apiBase + "/api/runs/" + id)
	if err != nil {
		h.logger.Error("failed to fetch run", "error", err)
		http.Error(w, "Failed to fetch run", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var run map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		h.logger.Error("failed to decode run", "error", err)
		http.Error(w, "Failed to decode run", http.StatusInternalServerError)
		return
	}

	h.tmpl.ExecuteTemplate(w, "run-detail.html", run)
}

func (h *Handler) cancelRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	req, err := http.NewRequest("POST", h.apiBase+"/api/runs/"+id+"/cancel", nil)
	if err != nil {
		h.logger.Error("failed to create cancel request", "error", err)
		http.Error(w, "Failed to cancel run", http.StatusInternalServerError)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.logger.Error("failed to cancel run", "error", err)
		http.Error(w, "Failed to cancel run", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		h.logger.Error("cancel failed", "status", resp.StatusCode, "body", string(body))
		http.Error(w, "Cancel failed", resp.StatusCode)
		return
	}

	w.Header().Set("HX-Trigger", "runCancelled")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) deleteSetup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	req, err := http.NewRequest("DELETE", h.apiBase+"/api/setups/"+id, nil)
	if err != nil {
		h.logger.Error("failed to create delete request", "error", err)
		http.Error(w, "Failed to delete setup", http.StatusInternalServerError)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.logger.Error("failed to delete setup", "error", err)
		http.Error(w, "Failed to delete setup", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		h.logger.Error("delete failed", "status", resp.StatusCode, "body", string(body))
		http.Error(w, "Delete failed", resp.StatusCode)
		return
	}

	w.Header().Set("HX-Trigger", "setupDeleted")
	w.WriteHeader(http.StatusOK)
}

func parseIntOrDefault(s string, def int) int {
	var val int
	if err := json.Unmarshal([]byte(s), &val); err != nil {
		return def
	}
	return val
}

func formatTime(t string) string {
	parsed, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		return t
	}
	return parsed.Format("Jan 02, 15:04:05")
}

func formatDuration(d interface{}) string {
	switch v := d.(type) {
	case string:
		dur, err := time.ParseDuration(v)
		if err != nil {
			return v
		}
		if dur < time.Second {
			return fmt.Sprintf("%.0fms", dur.Seconds()*1000)
		}
		return dur.Round(time.Millisecond).String()
	case float64:
		dur := time.Duration(v)
		if dur < time.Second {
			return fmt.Sprintf("%.0fms", dur.Seconds()*1000)
		}
		return dur.Round(time.Millisecond).String()
	default:
		return fmt.Sprintf("%v", d)
	}
}

func formatFloat(f float64) string {
	if f < 1 {
		return fmt.Sprintf("%.3f", f)
	}
	if f < 10 {
		return fmt.Sprintf("%.2f", f)
	}
	return fmt.Sprintf("%.1f", f)
}

func toJSON(v interface{}) template.JS {
	data, _ := json.Marshal(v)
	return template.JS(data)
}
