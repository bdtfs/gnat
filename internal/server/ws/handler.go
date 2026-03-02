package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bdtfs/gnat/internal/converters"
	"github.com/bdtfs/gnat/internal/models"
)

const (
	// How often to send stats updates to WebSocket clients.
	broadcastInterval = 1 * time.Second

	// Maximum time to wait for a pong response.
	pongWait = 60 * time.Second

	// Send pings at this interval (must be less than pongWait).
	pingInterval = 30 * time.Second

	// Maximum message size from client.
	maxMessageSize = 512
)

// wsMessage is the JSON envelope sent over WebSocket.
type wsMessage struct {
	Type string      `json:"type"` // "stats" or "completed"
	Data interface{} `json:"data"`
}

// RunGetter is the interface for getting run information.
type RunGetter interface {
	GetRun(id string) (*models.Run, error)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Handler handles WebSocket upgrade requests and streams stats.
type Handler struct {
	hub       *Hub
	runGetter RunGetter
	logger    *slog.Logger
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *Hub, runGetter RunGetter, logger *slog.Logger) *Handler {
	return &Handler{
		hub:       hub,
		runGetter: runGetter,
		logger:    logger,
	}
}

// HandleWebSocket upgrades an HTTP connection to WebSocket and streams
// run stats updates until the run completes or the client disconnects.
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	if runID == "" {
		http.Error(w, "run ID required", http.StatusBadRequest)
		return
	}

	// Verify the run exists.
	run, err := h.runGetter.GetRun(runID)
	if err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	// Upgrade to WebSocket.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws upgrade failed", "run_id", runID, "error", err)
		return
	}

	h.hub.Register(runID, conn)
	h.logger.Info("ws client connected", "run_id", runID)

	// If the run is already finished, send final stats and close.
	if isTerminal(run.Status) {
		h.sendFinalStats(conn, run)
		h.hub.Unregister(runID, conn)
		_ = conn.Close()
		return
	}

	// Start reading from the client (to detect disconnects).
	done := make(chan struct{})
	go h.readPump(conn, runID, done)

	// Stream stats until the run completes or client disconnects.
	h.streamStats(conn, runID, done)
}

// readPump reads from the WebSocket connection to detect client disconnect.
// It discards any incoming messages since the client only sends close frames.
func (h *Handler) readPump(conn *websocket.Conn, runID string, done chan struct{}) {
	defer close(done)
	defer func() {
		h.hub.Unregister(runID, conn)
		_ = conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Debug("ws read error", "run_id", runID, "error", err)
			}
			return
		}
	}
}

// streamStats sends periodic stats updates over WebSocket until the run
// completes or the client disconnects.
func (h *Handler) streamStats(conn *websocket.Conn, runID string, done <-chan struct{}) {
	ticker := time.NewTicker(broadcastInterval)
	defer ticker.Stop()

	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-ticker.C:
			run, err := h.runGetter.GetRun(runID)
			if err != nil {
				h.logger.Error("ws: failed to get run", "run_id", runID, "error", err)
				return
			}

			if isTerminal(run.Status) {
				h.sendFinalStats(conn, run)
				return
			}

			h.sendStatsUpdate(conn, run)
		}
	}
}

// sendStatsUpdate sends the current stats as a "stats" message.
func (h *Handler) sendStatsUpdate(conn *websocket.Conn, run *models.Run) {
	runDTO := converters.RunToDTO(run)
	msg := wsMessage{
		Type: "stats",
		Data: runDTO,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("ws: marshal stats failed", "error", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		h.logger.Debug("ws: write failed", "error", err)
	}
}

// sendFinalStats sends the final stats and status as a "completed" message.
func (h *Handler) sendFinalStats(conn *websocket.Conn, run *models.Run) {
	runDTO := converters.RunToDTO(run)
	msg := wsMessage{
		Type: "completed",
		Data: runDTO,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("ws: marshal final stats failed", "error", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		h.logger.Debug("ws: write final stats failed", "error", err)
	}
}

// isTerminal returns true if the run status indicates the run has finished.
func isTerminal(status models.RunStatus) bool {
	return status == models.RunStatusCompleted ||
		status == models.RunStatusFailed ||
		status == models.RunStatusCancelled
}
