// Package service coordinates the store.
package service

import "example.com/basic/store"

// Service places orders.
type Service struct {
	store *store.Store
}

// New returns a Service backed by st.
func New(st *store.Store) *Service {
	return &Service{store: st}
}

// Place saves an order and returns its identifier.
func (s *Service) Place(o store.Order) string {
	s.store.Save(o)
	return o.ID
}

// lookup is unexported, and is called only from within this package.
func (s *Service) lookup(id string) (store.Order, bool) {
	return s.store.Load(id)
}

// Exists reports whether an order is known.
func (s *Service) Exists(id string) bool {
	_, ok := s.lookup(id)
	return ok
}
