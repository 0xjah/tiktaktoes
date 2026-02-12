package ws

import (
	"net/http"

	"tiktaktoes/internal/broadcast"
	"tiktaktoes/internal/game"
	"tiktaktoes/internal/models"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Handler handles WebSocket connections for real-time game updates.
type Handler struct {
	gameService *game.Service
	hub         *broadcast.Hub
}

// NewHandler creates a new WebSocket handler.
func NewHandler(gameService *game.Service, hub *broadcast.Hub) *Handler {
	return &Handler{
		gameService: gameService,
		hub:         hub,
	}
}

// RegisterRoutes sets up the WebSocket routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ws/{gameID}", h.handleWebSocket)
}

func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	h.hub.RegisterWS(gameID, conn)
	defer h.hub.UnregisterWS(gameID, conn)

	// Send current game state
	if game, exists := h.gameService.GetGame(gameID); exists {
		conn.WriteJSON(game)
	}

	// Keep connection alive and listen for messages
	for {
		var move models.Move
		if err := conn.ReadJSON(&move); err != nil {
			break
		}
		if game, err := h.gameService.MakeMove(gameID, move); err == nil {
			h.hub.Broadcast(gameID, game)
		} else {
			conn.WriteJSON(map[string]string{"error": err.Error()})
		}
	}
}
