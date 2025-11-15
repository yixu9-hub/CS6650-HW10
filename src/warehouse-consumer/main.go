package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"
)

type OrderItem struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

type OrderMessage struct {
	OrderID string      `json:"orderId"`
	CartID  int         `json:"cart_id"`
	Items   []OrderItem `json:"items"`
}

var (
	totalOrders      int64
	countByProductID = make(map[string]int64)
	mu               sync.Mutex
)

func handleOrder(msg OrderMessage) {
	mu.Lock()
	defer mu.Unlock()

	totalOrders++
	for _, item := range msg.Items {
		countByProductID[item.ProductID] += int64(item.Quantity)
	}
}

func worker(conn *amqp.Connection, id int, wg *sync.WaitGroup) {
	defer wg.Done()

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("[worker %d] channel error: %v", id, err)
		return
	}
	defer ch.Close()

	ch.Qos(10, 0, false)

	msgs, err := ch.Consume("orders", "", false, false, false, false, nil)
	if err != nil {
		log.Printf("[worker %d] consume error: %v", id, err)
		return
	}

	log.Printf("[worker %d] start consuming", id)

	for d := range msgs {
		var order OrderMessage
		if err := json.Unmarshal(d.Body, &order); err != nil {
			d.Ack(false)
			continue
		}

		handleOrder(order)
		d.Ack(false)
	}
}

func main() {
	uri := os.Getenv("RABBITMQ_URI")
	if uri == "" {
		uri = "amqp://guest:guest@rabbitmq:5672/"
	}

	conn, err := amqp.Dial(uri)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// 并发 worker 数量（压测时会调）
	workerCount := 4
	if val := os.Getenv("WAREHOUSE_WORKERS"); val != "" {
		n, _ := strconv.Atoi(val)
		if n > 0 {
			workerCount = n
		}
	}

	var wg sync.WaitGroup
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go worker(conn, i, &wg)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down warehouse...")

	mu.Lock()
	log.Printf("Total Order number: %d", totalOrders)
	mu.Unlock()

	conn.Close()
	wg.Wait()
}
