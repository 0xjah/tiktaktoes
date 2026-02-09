package main

import (
	"log"
	"net/http"
	"tiktaktoes/internal/api"
	"tiktaktoes/internal/game"
)

func main() {
	// Initialize layers
	gameService := game.NewService()
	handler := api.NewHandler(gameService)

	// Setup routes
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Serve static files
	mux.Handle("/", http.FileServer(http.Dir("web")))

	// Apply CORS middleware
	server := api.CORSMiddleware(mux)

	log.Println("Server starting on http://localhost:8080/")
	log.Fatal(http.ListenAndServe(":8080", server))
}
