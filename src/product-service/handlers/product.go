package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"product-service/models"
	"product-service/storage"
)

// ErrorResponse models the error schema defined in the OpenAPI specification.
type ErrorResponse struct {
	Error   string  `json:"error"`
	Message string  `json:"message"`
	Details *string `json:"details,omitempty"`
}

// CreateProductResponse models the response for creating a product.
type CreateProductResponse struct {
	ProductID int `json:"product_id"`
}

// Handler exposes HTTP handlers for product operations.
type Handler struct {
	store *storage.MemoryStore
}

// NewHandler creates a Handler backed by the provided store.
func NewHandler(store *storage.MemoryStore) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes wires product routes onto the provided mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/product", h.handleCreateProduct)
	mux.HandleFunc("/products/", h.handleGetProduct)
	mux.HandleFunc("/health", h.handleHealth)
}

// handleHealth provides a health check endpoint for ALB.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// handleCreateProduct implements POST /product
// Per OpenAPI spec: server generates product_id, returns 201 with the ID.
func (h *Handler) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	// Only accept POST
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Parse request body
	var payload models.Product
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON payload")
		return
	}

	// Validate required fields (except product_id, which server generates)
	if err := validateCreateProductPayload(payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	// Server generates product_id
	productID := h.store.GenerateNextProductID()
	payload.ProductID = productID

	// Store product
	h.store.CreateProduct(payload)

	log.Printf("✅ Created product %d: %s by %s", productID, payload.SKU, payload.Manufacturer)

	// Return 201 Created with generated product_id
	writeJSON(w, http.StatusCreated, CreateProductResponse{
		ProductID: productID,
	})
}

// handleGetProduct implements GET /products/{productId}
// Per OpenAPI spec: returns product or 404 if not found.
func (h *Handler) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	// Only accept GET
	if r.Method != "GET" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Extract product ID from path: /products/{productId}
	path := strings.TrimPrefix(r.URL.Path, "/products/")
	if path == "" || strings.Contains(path, "/") {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid product ID in path")
		return
	}

	productID, err := parseProductID(path)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	// Retrieve product
	product, err := h.store.GetProduct(productID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Product not found")
			return
		}
		log.Printf("❌ ERROR: failed to retrieve product %d: %v", productID, err)
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve product")
		return
	}

	log.Printf("✅ Retrieved product %d: %s", productID, product.SKU)

	// Return 200 OK with product
	writeJSON(w, http.StatusOK, product)
}

// parseProductID parses and validates a product ID from a string.
func parseProductID(idStr string) (int, error) {
	if idStr == "" {
		return 0, errors.New("productId is required")
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, errors.New("productId must be an integer")
	}

	if id < 1 {
		return 0, errors.New("productId must be a positive integer")
	}

	return id, nil
}

// validateCreateProductPayload validates the product payload for creation.
// Does NOT validate product_id since server generates it.
func validateCreateProductPayload(product models.Product) error {
	// SKU validation
	if product.SKU == "" {
		return errors.New("sku is required")
	}
	if len(product.SKU) > 100 {
		return errors.New("sku must be 100 characters or fewer")
	}

	// Manufacturer validation
	if product.Manufacturer == "" {
		return errors.New("manufacturer is required")
	}
	if len(product.Manufacturer) > 200 {
		return errors.New("manufacturer must be 200 characters or fewer")
	}

	// Category ID validation
	if product.CategoryID < 1 {
		return errors.New("category_id must be a positive integer")
	}

	// Weight validation
	if product.Weight < 0 {
		return errors.New("weight must be non-negative")
	}

	// Some other ID validation
	if product.SomeOtherID < 1 {
		return errors.New("some_other_id must be a positive integer")
	}

	return nil
}

// writeError writes an error response in the OpenAPI-specified format.
func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:   code,
		Message: message,
	})
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}