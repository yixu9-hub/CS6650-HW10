package main

import (
	"log"
	"net/http"

	"credit-card-authorizer/handlers"
)

func main() {
	// Create handler
	handler := handlers.NewHandler()

	// Create mux
	mux := http.NewServeMux()

	// Register routes
	handler.RegisterRoutes(mux)

	// Start server
	log.Println("Starting credit card authorizer service on :8082")
	log.Fatal(http.ListenAndServe(":8082", mux))
}
