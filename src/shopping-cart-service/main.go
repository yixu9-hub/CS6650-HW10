package main

import (
	"log"
	"net/http"
	"os"

	"shopping-cart-service/handlers"
	"shopping-cart-service/storage"
)

func main() {
	// Get environment variables
	ccaURL := os.Getenv("CCA_URL")
	if ccaURL == "" {
		ccaURL = "http://localhost:8082"
	}

	// Create storage
	store := storage.NewMemoryStore()

	// Create handler
	handler := handlers.NewHandler(store, ccaURL)

	// Create mux
	mux := http.NewServeMux()

	// Register routes
	handler.RegisterRoutes(mux)

	// Start server
	log.Println("Starting shopping cart service on :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))
}
