package main

import (
	"log"
	"net/http"

	"product-service/handlers"
	"product-service/storage"
)

func main() {
	// Create storage
	store := storage.NewMemoryStore()

	// Create handler with 50% failure rate
	handler := handlers.NewHandler(store, 0.5)

	// Create mux
	mux := http.NewServeMux()

	// Register routes
	handler.RegisterRoutes(mux)

	// Start server
	log.Println("Starting BAD product service on :8080 (50% failure rate)")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
