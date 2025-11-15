package storage

import (
	"errors"
	"sync"

	"shopping-cart-service/models"
)

// Returned when a shopping cart cannot be found in the store.
var ErrNotFound = errors.New("shopping cart not found")

// Store defines the required operations for managing shopping carts.
// MemoryStore implements this interface.
type Store interface {
	// CreateCart creates a new shopping cart for the given customer
	// and returns the generated cart ID.
	CreateCart(customerID int) int

	// GetCart retrieves an existing cart. Returns ErrNotFound if missing.
	GetCart(cartID int) (*models.ShoppingCart, error)

	// AddItem adds a product to the cart or increases quantity if it already exists.
	AddItem(cartID, productID, quantity int) error

	// ClearCart removes all items from the specified cart.
	ClearCart(cartID int) error
}

// MemoryStore is an in-memory implementation of Store.
// It is safe for concurrent use via internal RWMutex.
type MemoryStore struct {
	carts  map[int]*models.ShoppingCart
	nextID int
	mu     sync.RWMutex
}

// NewMemoryStore creates and returns a new in-memory cart store.
// It returns the Store interface, hiding the concrete implementation.
func NewMemoryStore() Store {
	return &MemoryStore{
		carts:  make(map[int]*models.ShoppingCart),
		nextID: 1,
	}
}

// CreateCart creates a new shopping cart for the given customer ID.
// The cart ID is auto-incremented and returned to the caller.
func (s *MemoryStore) CreateCart(customerID int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cartID := s.nextID
	s.nextID++

	s.carts[cartID] = &models.ShoppingCart{
		ShoppingCartID: cartID,
		CustomerID:     customerID,
		Items:          []models.CartItem{},
	}

	return cartID
}

// GetCart returns the shopping cart with the specified ID.
// If the cart does not exist, ErrNotFound is returned.
func (s *MemoryStore) GetCart(cartID int) (*models.ShoppingCart, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cart, exists := s.carts[cartID]
	if !exists {
		return nil, ErrNotFound
	}
	return cart, nil
}

// AddItem adds a new item to the cart, or increments the quantity
// if the item already exists. Returns ErrNotFound if the cart doesn't exist.
func (s *MemoryStore) AddItem(cartID, productID, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, exists := s.carts[cartID]
	if !exists {
		return ErrNotFound
	}

	// Check if item already exists; if so, increase quantity.
	for i, item := range cart.Items {
		if item.ProductID == productID {
			cart.Items[i].Quantity += quantity
			return nil
		}
	}

	// Otherwise, add a new item to the cart.
	cart.Items = append(cart.Items, models.CartItem{
		ProductID: productID,
		Quantity:  quantity,
	})

	return nil
}

// ClearCart removes all items from the specified cart.
// Returns ErrNotFound if the cart does not exist.
func (s *MemoryStore) ClearCart(cartID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, exists := s.carts[cartID]
	if !exists {
		return ErrNotFound
	}

	cart.Items = []models.CartItem{}
	return nil
}
