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

// 和 Shopping Cart Service 发送的 JSON 对齐
type OrderItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

type OrderMessage struct {
	OrderID int         `json:"order_id"`
	CartID  int         `json:"cart_id"`
	Items   []OrderItem `json:"items"`
}

var (
	totalOrders      int64
	countByProductID = make(map[int]int64)
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

func worker(id int, msgs <-chan amqp.Delivery, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("[worker %d] started", id)

	for d := range msgs {
		var order OrderMessage
		if err := json.Unmarshal(d.Body, &order); err != nil {
			log.Printf("[worker %d] invalid JSON, ack and skip: %v", id, err)
			_ = d.Ack(false)
			continue
		}

		handleOrder(order)

		if err := d.Ack(false); err != nil {
			log.Printf("[worker %d] ack failed: %v", id, err)
		}
	}

	log.Printf("[worker %d] stopped (msgs channel closed)", id)
}

func main() {
	uri := os.Getenv("RABBITMQ_URI")
	if uri == "" {
		uri = "amqp://guest:guest@rabbitmq:5672/"
	}
	log.Printf("Connecting to RabbitMQ at %s", uri)

	conn, err := amqp.Dial(uri)
	if err != nil {
		// ❗ 不再 Fatalf，只打印错误并挂起，方便在本地或 CloudWatch 看日志
		log.Printf("Failed to connect to RabbitMQ: %v", err)
		waitForSignal()
		return
	}
	defer conn.Close()
	log.Println("Connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("Failed to open channel: %v", err)
		waitForSignal()
		return
	}
	defer ch.Close()

	// 这里先不 QueueDeclare，完全依赖 Shopping Cart 那边声明好的 "orders" 队列，
	// 避免 durable / autoDelete 参数不一致导致 PRECONDITION_FAILED。
	if err := ch.Qos(10, 0, false); err != nil {
		log.Printf("Failed to set QoS: %v", err)
		// 继续运行，只是没有预取优化
	}

	msgs, err := ch.Consume(
		"orders", // 队列名要和 SCS 使用的一致
		"",
		false, // 手动 ACK
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("Failed to start consuming: %v", err)
		waitForSignal()
		return
	}

	workerCount := 4
	if val := os.Getenv("WAREHOUSE_WORKERS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			workerCount = n
		}
	}
	log.Printf("Starting %d workers", workerCount)

	var wg sync.WaitGroup
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go worker(i, msgs, &wg)
	}

	waitForSignal()

	log.Println("Shutting down warehouse...")

	mu.Lock()
	log.Printf("Total Order number: %d", totalOrders)
	mu.Unlock()

	wg.Wait()
	log.Println("Warehouse stopped cleanly")
}

func waitForSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}