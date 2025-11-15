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

	// Create handler
	handler := handlers.NewHandler(store)

	// Create mux
	mux := http.NewServeMux()

	// Register routes
	handler.RegisterRoutes(mux)

	// Start server
	log.Println("Starting product service on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
