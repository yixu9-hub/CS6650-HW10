package storage

import (
	"errors"
	"sync"

	"product-service/models"
)

var (
	// ErrNotFound is returned when a product is not found.
	ErrNotFound = errors.New("product not found")
)

// MemoryStore provides in-memory storage for products.
type MemoryStore struct {
	mu            sync.RWMutex
	products      map[int]models.Product
	nextProductID int
}

// NewMemoryStore creates a new in-memory product store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		products:      make(map[int]models.Product),
		nextProductID: 1,
	}
}

// GenerateNextProductID generates and returns the next available product ID.
func (s *MemoryStore) GenerateNextProductID() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextProductID
	s.nextProductID++
	return id
}

// CreateProduct stores a new product and returns its ID.
func (s *MemoryStore) CreateProduct(product models.Product) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.products[product.ProductID] = product
	return product.ProductID
}

// GetProduct retrieves a product by its ID.
func (s *MemoryStore) GetProduct(productID int) (models.Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product, exists := s.products[productID]
	if !exists {
		return models.Product{}, ErrNotFound
	}

	return product, nil
}

// GetAllProducts returns all products (useful for debugging/testing).
func (s *MemoryStore) GetAllProducts() []models.Product {
	s.mu.RLock()
	defer s.mu.RUnlock()

	products := make([]models.Product, 0, len(s.products))
	for _, p := range s.products {
		products = append(products, p)
	}
	return products
}