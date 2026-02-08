package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"tiktaktoes/internal/game"
	"tiktaktoes/internal/models"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for simplicity
	},
}

// Handler handles HTTP requests
type Handler struct {
	gameService *game.Service
	clients     map[string]map[*websocket.Conn]bool
	mu          sync.RWMutex
}

// NewHandler creates a new handler
func NewHandler(gameService *game.Service) *Handler {
	return &Handler{
		gameService: gameService,
		clients:     make(map[string]map[*websocket.Conn]bool),
	}
}

// RegisterRoutes sets up the routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/game", h.handleCreateGame)
	mux.HandleFunc("/api/game/", h.handleGameActions)
	mux.HandleFunc("/ws/", h.handleWebSocket)
}

// handleCreateGame creates a new game
func (h *Handler) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	game := h.gameService.CreateGame()
	h.respondJSON(w, game)
}

// handleGameActions handles game-specific actions
func (h *Handler) handleGameActions(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Path[len("/api/game/"):]
	if gameID == "" {
		http.Error(w, "Game ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getGame(w, gameID)
	case http.MethodPost:
		h.makeMove(w, r, gameID)
	case http.MethodPut:
		h.resetGame(w, gameID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getGame(w http.ResponseWriter, gameID string) {
	game, exists := h.gameService.GetGame(gameID)
	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}
	h.respondJSON(w, game)
}

func (h *Handler) makeMove(w http.ResponseWriter, r *http.Request, gameID string) {
	var move models.Move
	if err := json.NewDecoder(r.Body).Decode(&move); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	game, err := h.gameService.MakeMove(gameID, move)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Broadcast to all connected clients
	h.broadcast(gameID, game)
	h.respondJSON(w, game)
}

func (h *Handler) resetGame(w http.ResponseWriter, gameID string) {
	game, err := h.gameService.ResetGame(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.broadcast(gameID, game)
	h.respondJSON(w, game)
}

// handleWebSocket handles WebSocket connections for real-time updates
func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Path[len("/ws/"):]
	if gameID == "" {
		http.Error(w, "Game ID required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Register client
	h.mu.Lock()
	if h.clients[gameID] == nil {
		h.clients[gameID] = make(map[*websocket.Conn]bool)
	}
	h.clients[gameID][conn] = true
	h.mu.Unlock()

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
			h.broadcast(gameID, game)
		} else {
			conn.WriteJSON(map[string]string{"error": err.Error()})
		}
	}

	// Unregister client
	h.mu.Lock()
	delete(h.clients[gameID], conn)
	h.mu.Unlock()
}

// broadcast sends game state to all connected clients
func (h *Handler) broadcast(gameID string, game *models.GameState) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.clients[gameID] {
		conn.WriteJSON(game)
	}
}

func (h *Handler) respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
