package handlers

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"regexp"
	"time"
)

type ErrorResponse struct {
	Error   string  `json:"error"`
	Message string  `json:"message"`
	Details *string `json:"details,omitempty"`
}

type Handler struct {
	rng *rand.Rand
}

func NewHandler() *Handler {
	return &Handler{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/credit-card-authorizer/authorize", h.handleAuthorize)
}

func (h *Handler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var payload struct {
		CreditCardNumber string `json:"credit_card_number"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON payload")
		return
	}

	// Validate credit card format: 4 groups of 4 digits separated by dashes
	validFormat := regexp.MustCompile(`^\d{4}-\d{4}-\d{4}-\d{4}$`)
	if !validFormat.MatchString(payload.CreditCardNumber) {
		h.writeError(w, http.StatusBadRequest, "INVALID_FORMAT", "Credit card number must be in format: 1234-5678-9012-3456")
		return
	}

	// Simulate authorization: 90% authorized, 10% declined
	authorized := h.rng.Float32() < 0.9

	if !authorized {
		h.writeError(w, http.StatusPaymentRequired, "PAYMENT_DECLINED", "Payment was declined")
		return
	}

	// Success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "Authorized"})
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: code, Message: message})
}
