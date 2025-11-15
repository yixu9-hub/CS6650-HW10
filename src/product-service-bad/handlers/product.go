package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"product-service/models"
	"product-service/storage"
)

// ErrorResponse models the error schema defined in the OpenAPI specification.
type ErrorResponse struct {
	Error   string  `json:"error"`
	Message string  `json:"message"`
	Details *string `json:"details,omitempty"`
}

// Handler exposes HTTP handlers for product operations.
type Handler struct {
	store      *storage.MemoryStore
	failureRate float32
	rng        *rand.Rand
}

// NewHandler creates a Handler backed by the provided store.
// failureRate: probability of returning 503 error (0.0 to 1.0)
func NewHandler(store *storage.MemoryStore, failureRate float32) *Handler {
	return &Handler{
		store:      store,
		failureRate: failureRate,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// RegisterRoutes wires product routes onto the provided mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/products/", h.handleProducts)
	mux.HandleFunc("/product", h.handleCreateProduct)
}

func (h *Handler) handleProducts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/products/")
	if path == "" {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Invalid path")
		return
	}
	
	if strings.Contains(path, "/") {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Invalid path")
		return
	}
	
	if r.Method != "GET" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}
	
	h.handleGetProduct(w, r, path)
}

func (h *Handler) handleGetProduct(w http.ResponseWriter, r *http.Request, idStr string) {
	// Simulate random failures if failure rate is set
	if h.failureRate > 0 && h.rng.Float32() < h.failureRate {
		log.Printf("SIMULATED FAILURE: Returning 503 for product %s", idStr)
		h.writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Service temporarily unavailable")
		return
	}

	productID, err := parseProductID(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	product, err := h.store.GetProduct(productID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "The requested product does not exist")
			return
		}
		log.Printf("ERROR: failed to retrieve product %d: %v", productID, err)
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve product")
		return
	}

	writeJSON(w, http.StatusOK, product)
}

func (h *Handler) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Simulate random failures if failure rate is set
	if h.failureRate > 0 && h.rng.Float32() < h.failureRate {
		log.Printf("SIMULATED FAILURE: Returning 503 for product creation")
		h.writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Service temporarily unavailable")
		return
	}

	var payload models.Product
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON payload")
		return
	}

	if validationErr := validateProductPayload(payload); validationErr != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", validationErr.Error())
		return
	}

	productID := h.store.CreateProduct(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"product_id": productID})
}

func parseProductID(idStr string) (int, error) {
	value := idStr
	if value == "" {
		return 0, errors.New("productId path parameter is required")
	}

	id, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New("productId must be an integer")
	}

	if id < 1 {
		return 0, errors.New("productId must be a positive integer")
	}

	return id, nil
}

func validateProductPayload(product models.Product) error {
	if product.SKU == "" {
		return errors.New("sku is required")
	}
	if len(product.SKU) > 100 {
		return errors.New("sku must be 100 characters or fewer")
	}
	if product.Manufacturer == "" {
		return errors.New("manufacturer is required")
	}
	if len(product.Manufacturer) > 200 {
		return errors.New("manufacturer must be 200 characters or fewer")
	}
	if product.CategoryID < 1 {
		return errors.New("category_id must be a positive integer")
	}
	if product.Weight < 0 {
		return errors.New("weight must be zero or a positive integer")
	}
	if product.SomeOtherID < 1 {
		return errors.New("some_other_id must be a positive integer")
	}

	return nil
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{Error: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
