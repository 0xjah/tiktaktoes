package main

import (
	"log"
	"net/http"
	"tiktaktoes/internal/api"
	"tiktaktoes/internal/broadcast"
	"tiktaktoes/internal/game"
	"tiktaktoes/internal/htmx"
	"tiktaktoes/internal/ws"
)

func main() {
	// Initialize shared services
	gameService := game.NewService()
	hub := broadcast.NewHub()

	// Initialize handlers
	apiHandler := api.NewHandler(gameService, hub)
	wsHandler := ws.NewHandler(gameService, hub)
	htmxHandler := htmx.NewHandler(gameService, hub)

	// Setup routes
	mux := http.NewServeMux()
	apiHandler.RegisterRoutes(mux)
	wsHandler.RegisterRoutes(mux)
	htmxHandler.RegisterRoutes(mux)

	// Serve static files
	mux.Handle("/", http.FileServer(http.Dir("web")))

	// Apply CORS middleware
	server := api.CORSMiddleware(mux)

	log.Println("Server starting on http://localhost:8080/")
	log.Fatal(http.ListenAndServe(":8080", server))
}
