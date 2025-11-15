package storage

import (
	"errors"
	"sync"
	"product-service/models"
)

// ErrNotFound is returned when a product is not found.
var ErrNotFound = errors.New("product not found")

// MemoryStore is an in-memory storage for products.
type MemoryStore struct {
	products map[int]models.Product
	nextID   int
	mu       sync.RWMutex
}

// NewMemoryStore creates a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		products: make(map[int]models.Product),
		nextID:   1,
	}
}

// GetProduct retrieves a product by ID.
func (s *MemoryStore) GetProduct(id int) (models.Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	product, exists := s.products[id]
	if !exists {
		return models.Product{}, ErrNotFound
	}
	return product, nil
}

// CreateProduct creates a new product and returns the generated ID.
func (s *MemoryStore) CreateProduct(product models.Product) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	productID := s.nextID
	s.nextID++
	
	product.ProductID = productID
	s.products[productID] = product
	
	return productID
}
