package models

// ShoppingCart represents a shopping cart.
type ShoppingCart struct {
	ShoppingCartID int         `json:"shopping_cart_id"`
	CustomerID     int         `json:"customer_id"`
	Items          []CartItem  `json:"items"`
}

// CartItem represents an item in the cart.
type CartItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}
