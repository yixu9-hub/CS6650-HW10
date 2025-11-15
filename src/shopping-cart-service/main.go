package main

import (
	"log"
	"net/http"
	"os"

	"shopping-cart-service/handlers"
	"shopping-cart-service/storage"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	// 1. Read CCA endpoint
	ccaURL := os.Getenv("CCA_URL")
	if ccaURL == "" {
		ccaURL = "http://localhost:8082"
	}

	// 2. Connect to RabbitMQ
	rabbitURI := os.Getenv("RABBITMQ_URI")
	if rabbitURI == "" {
		// Default for local docker-compose; AWS will override via environment variables
		rabbitURI = "amqp://guest:guest@rabbitmq:5672/"
	}

	conn, err := amqp.Dial(rabbitURI)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open RabbitMQ channel: %v", err)
	}
	defer ch.Close()

	// Declare the order queue (consumed by Warehouse)
	q, err := ch.QueueDeclare(
		"orders",
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
	if err != nil {
		log.Fatalf("failed to declare orders queue: %v", err)
	}

	// 3. Initialize in-memory storage
	store := storage.NewMemoryStore()

	// 4. Create handler — passing storage, CCA URL, and RabbitMQ components
	handler := handlers.NewHandler(store, ccaURL, ch, q.Name)

	// 5. Register HTTP routes
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 6. Start HTTP server
	addr := ":8081"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Println("Starting shopping cart service on", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
