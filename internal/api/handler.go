package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"text/template"
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
	sseClients  map[string]map[chan *models.GameState]bool
	mu          sync.RWMutex
}

// NewHandler creates a new handler
func NewHandler(gameService *game.Service) *Handler {
	return &Handler{
		gameService: gameService,
		clients:     make(map[string]map[*websocket.Conn]bool),
		sseClients:  make(map[string]map[chan *models.GameState]bool),
	}
}

// RegisterRoutes sets up the routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/game", h.handleCreateGame)
	mux.HandleFunc("/api/game/", h.handleGameActions)
	mux.HandleFunc("/ws/", h.handleWebSocket)
	// HTMX routes
	mux.HandleFunc("/htmx/game/new", h.htmxNewGame)
	mux.HandleFunc("/htmx/game", h.htmxGetGame)
	mux.HandleFunc("/htmx/move/", h.htmxMakeMove)
	mux.HandleFunc("/htmx/reset/", h.htmxResetGame)
	mux.HandleFunc("/htmx/sse/", h.htmxSSE)
}

// handleCreateGame creates a new game
func (h *Handler) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	game := h.gameService.CreateGame(models.Empty)
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

	// Broadcast to SSE clients
	for ch := range h.sseClients[gameID] {
		select {
		case ch <- game:
		default:
		}
	}
}

func (h *Handler) respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// HTMX Handlers

// getPlayerFromRequest gets the player from either form values or query params
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

func (h *Handler) htmxNewGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	player := getPlayerFromRequest(r)
	game := h.gameService.CreateGame(models.Player(player))
	h.renderGameHTML(w, game, player)
}

func (h *Handler) htmxGetGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("gameId")
	if gameID == "" {
		gameID = r.FormValue("gameId")
	}
	if gameID == "" {
		http.Error(w, "Game ID required", http.StatusBadRequest)
		return
	}

	player := getPlayerFromRequest(r)

	// Try to join the game
	game, err := h.gameService.JoinGame(gameID, models.Player(player))
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fmt.Sprintf(`<div class="status" id="status">&gt; error: %s</div>`, template.HTMLEscapeString(err.Error()))))
		return
	}

	h.renderGameHTML(w, game, player)
}

func (h *Handler) htmxMakeMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/htmx/move/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	gameID := parts[0]
	var position int
	fmt.Sscanf(parts[1], "%d", &position)

	player := getPlayerFromRequest(r)

	move := models.Move{
		Position: position,
		Player:   models.Player(player),
	}

	game, err := h.gameService.MakeMove(gameID, move)
	if err != nil {
		// Return current state with error message
		game, _ = h.gameService.GetGame(gameID)
		if game != nil {
			h.renderGameHTML(w, game, player)
		}
		return
	}

	h.broadcast(gameID, game)
	h.renderGameHTML(w, game, player)
}

func (h *Handler) htmxResetGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	gameID := strings.TrimPrefix(r.URL.Path, "/htmx/reset/")
	// Remove query string from gameID if present
	if idx := strings.Index(gameID, "?"); idx != -1 {
		gameID = gameID[:idx]
	}

	player := getPlayerFromRequest(r)

	game, err := h.gameService.ResetGame(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.broadcast(gameID, game)
	h.renderGameHTML(w, game, player)
}

func (h *Handler) htmxSSE(w http.ResponseWriter, r *http.Request) {
	gameID := strings.TrimPrefix(r.URL.Path, "/htmx/sse/")
	// Remove query string from gameID if present
	if idx := strings.Index(gameID, "?"); idx != -1 {
		gameID = gameID[:idx]
	}
	if gameID == "" {
		http.Error(w, "Game ID required", http.StatusBadRequest)
		return
	}

	player := r.URL.Query().Get("player")
	if player == "" {
		player = "X"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan *models.GameState, 10)

	h.mu.Lock()
	if h.sseClients[gameID] == nil {
		h.sseClients[gameID] = make(map[chan *models.GameState]bool)
	}
	h.sseClients[gameID][ch] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.sseClients[gameID], ch)
		h.mu.Unlock()
		close(ch)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Send initial state
	if game, exists := h.gameService.GetGame(gameID); exists {
		html := h.getGameContentHTML(game, player)
		fmt.Fprintf(w, "event: game-update\ndata: %s\n\n", strings.ReplaceAll(html, "\n", ""))
		flusher.Flush()
	}

	for {
		select {
		case game := <-ch:
			html := h.getGameContentHTML(game, player)
			fmt.Fprintf(w, "event: game-update\ndata: %s\n\n", strings.ReplaceAll(html, "\n", ""))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handler) renderGameHTML(w http.ResponseWriter, game *models.GameState, player string) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(h.getGameWrapperHTML(game, player)))
}

// getGameWrapperHTML returns the full game HTML with SSE wrapper (for initial load)
func (h *Handler) getGameWrapperHTML(game *models.GameState, player string) string {
	return fmt.Sprintf(`<div hx-ext="sse" sse-connect="/htmx/sse/%s?player=%s" sse-swap="game-update" hx-swap="innerHTML" data-game-id="%s">
<div id="game-content">%s</div>
</div>`,
		template.HTMLEscapeString(game.ID),
		template.HTMLEscapeString(player),
		template.HTMLEscapeString(game.ID),
		h.getGameContentHTML(game, player),
	)
}

// getGameContentHTML returns just the inner game content (for SSE updates)
func (h *Handler) getGameContentHTML(game *models.GameState, player string) string {
	var status string
	if game.IsOver {
		if game.IsDraw {
			status = "&gt; result: draw"
		} else {
			status = fmt.Sprintf("&gt; winner: %s", game.Winner)
		}
	} else {
		if string(game.CurrentTurn) == player {
			status = "&gt; your_turn"
		} else {
			status = fmt.Sprintf("&gt; waiting: %s...", game.CurrentTurn)
		}
	}

	var cells strings.Builder
	for i, cell := range game.Board {
		cellClass := "cell"
		cellContent := ""
		hxAttrs := ""

		switch cell {
		case models.PlayerX:
			cellClass += " x"
			cellContent = "X"
		case models.PlayerO:
			cellClass += " o"
			cellContent = "O"
		}

		// Add htmx attributes for empty cells when game is active
		if cell == models.Empty && !game.IsOver {
			hxAttrs = fmt.Sprintf(` hx-post="/htmx/move/%s/%d?player=%s" hx-target="#game-container" hx-swap="innerHTML"`,
				template.HTMLEscapeString(game.ID), i, template.HTMLEscapeString(player))
		} else {
			cellClass += " disabled"
		}

		cells.WriteString(fmt.Sprintf(`<div class="%s"%s>%s</div>`, cellClass, hxAttrs, cellContent))
	}

	return fmt.Sprintf(`<div class="status" id="status">%s</div>
<div class="board" id="board">%s</div>
<button class="btn" hx-post="/htmx/game/new?player=%s" hx-target="#game-container" hx-swap="innerHTML">[new]</button>
<button class="btn" hx-post="/htmx/reset/%s?player=%s" hx-target="#game-container" hx-swap="innerHTML">[reset]</button>
<div class="game-id" id="gameId">session: %s</div>
<div class="share-link" id="shareLink" onclick="copyShareLink('%s')">[click to copy link]</div>`,
		status,
		cells.String(),
		template.HTMLEscapeString(player),
		template.HTMLEscapeString(game.ID),
		template.HTMLEscapeString(player),
		template.HTMLEscapeString(game.ID),
		template.HTMLEscapeString(game.ID),
	)
}
