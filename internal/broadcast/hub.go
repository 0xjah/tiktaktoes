package broadcast

import (
	"sync"

	"tiktaktoes/internal/models"

	"github.com/gorilla/websocket"
)

// Hub manages broadcasting game state updates to WebSocket and SSE clients.
type Hub struct {
	wsClients  map[string]map[*websocket.Conn]bool
	sseClients map[string]map[chan *models.GameState]bool
	mu         sync.RWMutex
}

// NewHub creates a new broadcast hub.
func NewHub() *Hub {
	return &Hub{
		wsClients:  make(map[string]map[*websocket.Conn]bool),
		sseClients: make(map[string]map[chan *models.GameState]bool),
	}
}

// RegisterWS adds a WebSocket connection for a game.
func (h *Hub) RegisterWS(gameID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.wsClients[gameID] == nil {
		h.wsClients[gameID] = make(map[*websocket.Conn]bool)
	}
	h.wsClients[gameID][conn] = true
}

// UnregisterWS removes a WebSocket connection for a game.
func (h *Hub) UnregisterWS(gameID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.wsClients[gameID], conn)
}

// RegisterSSE adds an SSE channel for a game.
func (h *Hub) RegisterSSE(gameID string, ch chan *models.GameState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sseClients[gameID] == nil {
		h.sseClients[gameID] = make(map[chan *models.GameState]bool)
	}
	h.sseClients[gameID][ch] = true
}

// UnregisterSSE removes an SSE channel for a game.
func (h *Hub) UnregisterSSE(gameID string, ch chan *models.GameState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sseClients[gameID], ch)
	close(ch)
}

// Broadcast sends a game state update to all connected WebSocket and SSE clients.
func (h *Hub) Broadcast(gameID string, game *models.GameState) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.wsClients[gameID] {
		conn.WriteJSON(game)
	}
	for ch := range h.sseClients[gameID] {
		select {
		case ch <- game:
		default:
		}
	}
}
