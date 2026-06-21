package ws

import (
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub manages WebSocket connections grouped by run ID.
// It is thread-safe and supports broadcasting messages to all
// clients watching a specific run.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*websocket.Conn]struct{}
	logger  *slog.Logger
}

// NewHub creates a new WebSocket hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[string]map[*websocket.Conn]struct{}),
		logger:  logger,
	}
}

// Register adds a WebSocket connection as a subscriber for the given run ID.
func (h *Hub) Register(runID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[runID] == nil {
		h.clients[runID] = make(map[*websocket.Conn]struct{})
	}
	h.clients[runID][conn] = struct{}{}

	h.logger.Debug("ws client registered", "run_id", runID, "clients", len(h.clients[runID]))
}

// Unregister removes a WebSocket connection from the given run ID's subscribers.
func (h *Hub) Unregister(runID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns, ok := h.clients[runID]
	if !ok {
		return
	}

	delete(conns, conn)

	if len(conns) == 0 {
		delete(h.clients, runID)
	}

	h.logger.Debug("ws client unregistered", "run_id", runID)
}

// Broadcast sends a message to all clients watching the given run ID.
// Connections that fail to receive the message are removed.
func (h *Hub) Broadcast(runID string, message []byte) {
	h.mu.RLock()
	conns, ok := h.clients[runID]
	if !ok || len(conns) == 0 {
		h.mu.RUnlock()
		return
	}

	// Copy the connection set under read lock to avoid holding it while writing.
	targets := make([]*websocket.Conn, 0, len(conns))
	for conn := range conns {
		targets = append(targets, conn)
	}
	h.mu.RUnlock()

	failed := h.sendToTargets(runID, targets, message)
	h.dropConns(runID, failed)
}

func (h *Hub) sendToTargets(runID string, targets []*websocket.Conn, message []byte) []*websocket.Conn {
	var failed []*websocket.Conn
	for _, conn := range targets {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			h.logger.Debug("ws write failed, removing client", "run_id", runID, "error", err)
			failed = append(failed, conn)
		}
	}
	return failed
}

func (h *Hub) dropConns(runID string, failed []*websocket.Conn) {
	if len(failed) == 0 {
		return
	}

	h.mu.Lock()
	for _, conn := range failed {
		if connsMap, exists := h.clients[runID]; exists {
			delete(connsMap, conn)
			if len(connsMap) == 0 {
				delete(h.clients, runID)
			}
		}
		_ = conn.Close()
	}
	h.mu.Unlock()
}

// CleanupRun removes all connections for a given run ID and closes them.
func (h *Hub) CleanupRun(runID string) {
	h.mu.Lock()
	conns, ok := h.clients[runID]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.clients, runID)
	h.mu.Unlock()

	for conn := range conns {
		_ = conn.Close()
	}

	h.logger.Debug("ws cleanup complete", "run_id", runID)
}

// ClientCount returns the number of connected clients for a run ID.
func (h *Hub) ClientCount(runID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[runID])
}
