// Package store keeps orders.
package store

import "example.com/basic/internal/util"

// Order is one order.
type Order struct {
	ID   string
	Name string
}

// Store holds orders in memory.
type Store struct {
	orders map[string]Order
}

// New returns an empty Store.
func New() *Store {
	return &Store{orders: map[string]Order{}}
}

// Save records an order under a normalised key.
func (s *Store) Save(o Order) {
	s.orders[util.Normalize(o.ID)] = o
}

// Load returns the order under a key, and whether it was there.
func (s *Store) Load(id string) (Order, bool) {
	o, ok := s.orders[util.Normalize(id)]
	return o, ok
}
