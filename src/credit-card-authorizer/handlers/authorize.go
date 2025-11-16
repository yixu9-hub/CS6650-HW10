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
	// ✅ OpenAPI 里定义的路径
	mux.HandleFunc("/credit-card-authorizer/authorize", h.handleAuthorize)

	// （可选）给你自己 curl 用的短路径，不影响 YAML 一致性
	mux.HandleFunc("/authorize", h.handleAuthorize)
}

func (h *Handler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	// 校验格式：4 组 4 位数字，用短横线分隔
	validFormat := regexp.MustCompile(`^\d{4}-\d{4}-\d{4}-\d{4}$`)
	if !validFormat.MatchString(payload.CreditCardNumber) {
		// ✅ YAML: 400 Invalid payment information
		h.writeError(w, http.StatusBadRequest, "INVALID_FORMAT", "Credit card number must be in format: 1234-5678-9012-3456")
		return
	}

	// 90% 授权，10% 拒绝
	authorized := h.rng.Float32() < 0.9

	if !authorized {
		// ✅ YAML: 402 Payment declined
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "PAYMENT_DECLINED",
			Message: "Payment was declined",
		})
		return
	}

	// ✅ YAML: 200 Payment authorized successfully
	// Body YAML 没规定，你可以随意；这里给个简单 JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "Authorized",
	})
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: code, Message: message})
}