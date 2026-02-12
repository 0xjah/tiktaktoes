package htmx

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"tiktaktoes/internal/broadcast"
	"tiktaktoes/internal/game"
	"tiktaktoes/internal/models"

	"github.com/a-h/templ"
)

// Handler handles HTMX requests with SSE for real-time updates.
type Handler struct {
	gameService *game.Service
	hub         *broadcast.Hub
}

// NewHandler creates a new HTMX handler.
func NewHandler(gameService *game.Service, hub *broadcast.Hub) *Handler {
	return &Handler{
		gameService: gameService,
		hub:         hub,
	}
}

// RegisterRoutes sets up the HTMX routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /htmx/game/new", h.handleNewGame)
	mux.HandleFunc("/htmx/game", h.handleGetGame)
	mux.HandleFunc("POST /htmx/move/{gameID}/{position}", h.handleMakeMove)
	mux.HandleFunc("POST /htmx/reset/{gameID}", h.handleResetGame)
	mux.HandleFunc("/htmx/sse/{gameID}", h.handleSSE)
}

func getPlayerFromRequest(r *http.Request) string {
	r.ParseForm()
	player := r.FormValue("player")
	if player == "" {
		player = r.URL.Query().Get("player")
	}
	if player == "" {
		player = "X"
	}
	return player
}

func (h *Handler) handleNewGame(w http.ResponseWriter, r *http.Request) {
	player := getPlayerFromRequest(r)
	g := h.gameService.CreateGame(models.Player(player))
	w.Header().Set("Content-Type", "text/html")
	GameWrapper(g, player).Render(r.Context(), w)
}

func (h *Handler) handleGetGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("gameId")
	if gameID == "" {
		gameID = r.FormValue("gameId")
	}
	if gameID == "" {
		http.Error(w, "Game ID required", http.StatusBadRequest)
		return
	}
	player := getPlayerFromRequest(r)
	g, err := h.gameService.JoinGame(gameID, models.Player(player))
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		ErrorStatus(err.Error()).Render(r.Context(), w)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	GameWrapper(g, player).Render(r.Context(), w)
}

func (h *Handler) handleMakeMove(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	var position int
	fmt.Sscanf(r.PathValue("position"), "%d", &position)
	player := getPlayerFromRequest(r)
	move := models.Move{
		Position: position,
		Player:   models.Player(player),
	}
	g, err := h.gameService.MakeMove(gameID, move)
	if err != nil {
		g, _ = h.gameService.GetGame(gameID)
		if g != nil {
			w.Header().Set("Content-Type", "text/html")
			GameWrapper(g, player).Render(r.Context(), w)
		}
		return
	}
	h.hub.Broadcast(gameID, g)
	w.Header().Set("Content-Type", "text/html")
	GameWrapper(g, player).Render(r.Context(), w)
}

func (h *Handler) handleResetGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	player := getPlayerFromRequest(r)
	g, err := h.gameService.ResetGame(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.hub.Broadcast(gameID, g)
	w.Header().Set("Content-Type", "text/html")
	GameWrapper(g, player).Render(r.Context(), w)
}

func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	player := r.URL.Query().Get("player")
	if player == "" {
		player = "X"
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan *models.GameState, 10)
	h.hub.RegisterSSE(gameID, ch)
	defer h.hub.UnregisterSSE(gameID, ch)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}
	// Send initial state
	if g, exists := h.gameService.GetGame(gameID); exists {
		html := renderToString(r.Context(), GameContent(g, player))
		fmt.Fprintf(w, "event: game-update\ndata: %s\n\n", strings.ReplaceAll(html, "\n", ""))
		flusher.Flush()
	}
	for {
		select {
		case g := <-ch:
			html := renderToString(r.Context(), GameContent(g, player))
			fmt.Fprintf(w, "event: game-update\ndata: %s\n\n", strings.ReplaceAll(html, "\n", ""))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func renderToString(ctx context.Context, component templ.Component) string {
	var buf bytes.Buffer
	component.Render(ctx, &buf)
	return buf.String()
}
