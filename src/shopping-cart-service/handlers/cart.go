package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"

	"shopping-cart-service/models"
	"shopping-cart-service/storage"
)

// ErrorResponse represents a unified error response format.
type ErrorResponse struct {
	Error   string  `json:"error"`
	Message string  `json:"message"`
	Details *string `json:"details,omitempty"`
}

// Handler holds dependencies for the shopping cart service:
// - storage layer
// - credit card authorizer endpoint
// - RabbitMQ channel and queue info
type Handler struct {
	store       storage.Store
	ccaURL      string
	nextOrderID int

	mqChannel *amqp.Channel
	queueName string
}

// OrderMessage represents the payload sent to RabbitMQ when an order is created.
type OrderMessage struct {
	OrderID int               `json:"order_id"`
	CartID  int               `json:"cart_id"`
	Items   []models.CartItem `json:"items"`
}

// NewHandler constructs the handler with storage, CCA URL, and RabbitMQ components.
func NewHandler(store storage.Store, ccaURL string, ch *amqp.Channel, queueName string) *Handler {
	return &Handler{
		store:       store,
		ccaURL:      ccaURL,
		nextOrderID: 1,
		mqChannel:   ch,
		queueName:   queueName,
	}
}

// RegisterRoutes maps the HTTP endpoints to handler functions.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/shopping-cart", h.handleCreateCart)
	mux.HandleFunc("/shopping-carts/", h.handleCartOperations)
}

// handleCreateCart creates a new shopping cart.
func (h *Handler) handleCreateCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var payload struct {
		CustomerID int `json:"customer_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON payload")
		return
	}

	if payload.CustomerID < 1 {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "customer_id must be a positive integer")
		return
	}

	cartID := h.store.CreateCart(payload.CustomerID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]int{"shopping_cart_id": cartID})
}

// handleCartOperations dispatches operations like addItem and checkout.
func (h *Handler) handleCartOperations(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/shopping-carts/")

	if strings.HasSuffix(path, "/addItem") {
		idStr := strings.TrimSuffix(path, "/addItem")
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		h.handleAddItem(w, r, idStr)

	} else if strings.HasSuffix(path, "/checkout") {
		idStr := strings.TrimSuffix(path, "/checkout")
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		h.handleCheckout(w, r, idStr)

	} else {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
	}
}

// handleAddItem adds an item or increases its quantity in a shopping cart.
func (h *Handler) handleAddItem(w http.ResponseWriter, r *http.Request, idStr string) {
	cartID, err := strconv.Atoi(idStr)
	if err != nil || cartID < 1 {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid shopping cart ID")
		return
	}

	var payload struct {
		ProductID int `json:"product_id"`
		Quantity  int `json:"quantity"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON payload")
		return
	}

	if payload.ProductID < 1 {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "product_id must be a positive integer")
		return
	}

	if payload.Quantity < 1 {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "quantity must be a positive integer")
		return
	}

	if err := h.store.AddItem(cartID, payload.ProductID, payload.Quantity); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.writeError(w, http.StatusNotFound, "CART_NOT_FOUND", "Shopping cart not found")
			return
		}
		log.Printf("ERROR: failed to add item: %v", err)
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to add item")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleCheckout authorizes payment and publishes the order to RabbitMQ.
func (h *Handler) handleCheckout(w http.ResponseWriter, r *http.Request, idStr string) {
	cartID, err := strconv.Atoi(idStr)
	if err != nil || cartID < 1 {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid shopping cart ID")
		return
	}

	var payload struct {
		CreditCardNumber string `json:"credit_card_number"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON payload")
		return
	}

	// Retrieve the shopping cart
	cart, err := h.store.GetCart(cartID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.writeError(w, http.StatusNotFound, "CART_NOT_FOUND", "Shopping cart not found")
			return
		}
		log.Printf("ERROR: failed to get cart: %v", err)
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve cart")
		return
	}

	if len(cart.Items) == 0 {
		h.writeError(w, http.StatusBadRequest, "EMPTY_CART", "Cannot checkout empty cart")
		return
	}

	// Contact CCA for payment authorization
	authorized, err := h.authorizePayment(payload.CreditCardNumber)
	if err != nil {
		// 400 场景（格式错、其他异常） → INVALID_CARD
		h.writeError(w, http.StatusBadRequest, "INVALID_CARD", err.Error())
		return
	}
	if !authorized {
		// 402 场景（支付拒绝）
		h.writeError(w, http.StatusPaymentRequired, "PAYMENT_DECLINED", "Payment was declined")
		return
	}

	// Generate a new order ID
	orderID := h.nextOrderID
	h.nextOrderID++

	// Create the message payload to send to RabbitMQ
	msg := OrderMessage{
		OrderID: orderID,
		CartID:  cartID,
		Items:   cart.Items,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ERROR: failed to marshal order message: %v", err)
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to enqueue order")
		return
	}

	// Publish the message to RabbitMQ (fire-and-forget)
	if err := h.mqChannel.Publish(
		"",
		h.queueName,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	); err != nil {
		log.Printf("ERROR: failed to publish order to RabbitMQ: %v", err)
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to enqueue order")
		return
	}

	// Clear the cart after successful order submission
	_ = h.store.ClearCart(cartID)

	log.Printf("Order %d created for cart %d with %d items", orderID, cartID, len(cart.Items))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]int{"order_id": orderID})
}

// authorizePayment calls the credit card authorizer service.
//
// 协议和 OpenAPI 对齐：
//   - 200 OK  → 授权成功
//   - 400 Bad Request → 卡号格式错误
//   - 402 Payment Required → 授权被拒
func (h *Handler) authorizePayment(cardNumber string) (bool, error) {
    reqBody, _ := json.Marshal(map[string]string{
        "credit_card_number": cardNumber,
    })

    // ★ 关键改动：不再自己拼路径，直接用环境变量里的完整 URL
    resp, err := http.Post(h.ccaURL, "application/json", bytes.NewBuffer(reqBody))
    if err != nil {
        return false, fmt.Errorf("failed to contact payment service")
    }
    defer resp.Body.Close()

    // ★ 为了 debug 更清楚，把 status code 带进错误信息里
    if resp.StatusCode == http.StatusBadRequest {
        return false, fmt.Errorf("invalid credit card format")
    }

    if resp.StatusCode == http.StatusPaymentRequired {
        // 402 → 拒绝
        return false, nil
    }

    if resp.StatusCode == http.StatusOK {
        // 200 → 授权通过
        return true, nil
    }

    // 其他情况，一律认为是“payment service 返回了意外状态码”
    return false, fmt.Errorf("unexpected response from payment service (status %d)", resp.StatusCode)
}

// writeError writes a standardized error JSON response.
func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: code, Message: message})
}