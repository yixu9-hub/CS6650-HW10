package storage

import (
	"errors"
	"sync"
	"shopping-cart-service/models"
)

var ErrNotFound = errors.New("shopping cart not found")

type MemoryStore struct {
	carts      map[int]*models.ShoppingCart
	nextID     int
	mu         sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		carts:  make(map[int]*models.ShoppingCart),
		nextID: 1,
	}
}

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

func (s *MemoryStore) GetCart(cartID int) (*models.ShoppingCart, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	cart, exists := s.carts[cartID]
	if !exists {
		return nil, ErrNotFound
	}
	
	return cart, nil
}

func (s *MemoryStore) AddItem(cartID, productID, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	cart, exists := s.carts[cartID]
	if !exists {
		return ErrNotFound
	}
	
	// Check if item already exists, update quantity
	for i, item := range cart.Items {
		if item.ProductID == productID {
			cart.Items[i].Quantity += quantity
			return nil
		}
	}
	
	// Add new item
	cart.Items = append(cart.Items, models.CartItem{
		ProductID: productID,
		Quantity:  quantity,
	})
	
	return nil
}

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
