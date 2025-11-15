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

	"shopping-cart-service/storage"
)

type ErrorResponse struct {
	Error   string  `json:"error"`
	Message string  `json:"message"`
	Details *string `json:"details,omitempty"`
}

type Handler struct {
	store       *storage.MemoryStore
	ccaURL      string
	nextOrderID int
}

func NewHandler(store *storage.MemoryStore, ccaURL string) *Handler {
	return &Handler{
		store:       store,
		ccaURL:      ccaURL,
		nextOrderID: 1,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/shopping-cart", h.handleCreateCart)
	mux.HandleFunc("/shopping-carts/", h.handleCartOperations)
}

func (h *Handler) handleCreateCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
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
	json.NewEncoder(w).Encode(map[string]int{"shopping_cart_id": cartID})
}

func (h *Handler) handleCartOperations(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/shopping-carts/")
	
	if strings.HasSuffix(path, "/addItem") {
		idStr := strings.TrimSuffix(path, "/addItem")
		if r.Method != "POST" {
			h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		h.handleAddItem(w, r, idStr)
	} else if strings.HasSuffix(path, "/checkout") {
		idStr := strings.TrimSuffix(path, "/checkout")
		if r.Method != "POST" {
			h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		h.handleCheckout(w, r, idStr)
	} else {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
	}
}

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

	err = h.store.AddItem(cartID, payload.ProductID, payload.Quantity)
	if err != nil {
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

	// Get cart
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

	// Authorize payment with CCA
	authorized, err := h.authorizePayment(payload.CreditCardNumber)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_CARD", err.Error())
		return
	}

	if !authorized {
		h.writeError(w, http.StatusPaymentRequired, "PAYMENT_DECLINED", "Payment was declined")
		return
	}

	// Generate order ID
	orderID := h.nextOrderID
	h.nextOrderID++

	// Clear cart
	h.store.ClearCart(cartID)
	
	log.Printf("Order %d created for cart %d with %d items", orderID, cartID, len(cart.Items))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"order_id": orderID})
}

func (h *Handler) authorizePayment(cardNumber string) (bool, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"credit_card_number": cardNumber,
	})

	resp, err := http.Post(h.ccaURL+"/credit-card-authorizer/authorize", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return false, fmt.Errorf("failed to contact payment service")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return false, fmt.Errorf("invalid credit card format")
	}

	if resp.StatusCode == http.StatusPaymentRequired {
		return false, nil // Declined
	}

	if resp.StatusCode == http.StatusOK {
		return true, nil // Authorized
	}

	return false, fmt.Errorf("unexpected response from payment service")
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: code, Message: message})
}
