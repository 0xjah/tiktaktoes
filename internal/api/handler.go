package api

import (
	"encoding/json"
	"net/http"
	"tiktaktoes/internal/broadcast"
	"tiktaktoes/internal/game"
	"tiktaktoes/internal/models"
)

// Handler handles REST API requests.
type Handler struct {
	gameService *game.Service
	hub         *broadcast.Hub
}

// NewHandler creates a new REST API handler.
func NewHandler(gameService *game.Service, hub *broadcast.Hub) *Handler {
	return &Handler{
		gameService: gameService,
		hub:         hub,
	}
}

// RegisterRoutes sets up the REST API routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/game", h.handleCreateGame)
	mux.HandleFunc("GET /api/game/{gameID}", h.handleGetGame)
	mux.HandleFunc("POST /api/game/{gameID}", h.handleMakeMove)
	mux.HandleFunc("PUT /api/game/{gameID}", h.handleResetGame)
}

func (h *Handler) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	g := h.gameService.CreateGame(models.Empty)
	respondJSON(w, g)
}

func (h *Handler) handleGetGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	g, exists := h.gameService.GetGame(gameID)
	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}
	respondJSON(w, g)
}

func (h *Handler) handleMakeMove(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	var move models.Move
	if err := json.NewDecoder(r.Body).Decode(&move); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	g, err := h.gameService.MakeMove(gameID, move)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.hub.Broadcast(gameID, g)
	respondJSON(w, g)
}

func (h *Handler) handleResetGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	g, err := h.gameService.ResetGame(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.hub.Broadcast(gameID, g)
	respondJSON(w, g)
}

func respondJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
