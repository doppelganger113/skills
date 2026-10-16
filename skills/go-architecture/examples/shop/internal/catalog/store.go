package catalog

import (
	"context"
	"sync"
)

// Store persists products.
//
// In a real service this wraps *sql.DB or sqlc-generated queries and
// translates sql.ErrNoRows into ErrNotFound. It is in-memory here so the
// example has no external dependencies; the method set and error
// contract are what a Postgres store would expose.
type Store struct {
	mu       sync.RWMutex
	products map[string]Product
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{products: make(map[string]Product)}
}

// Insert adds p, returning ErrDuplicateSKU if the SKU already exists.
func (s *Store) Insert(_ context.Context, p Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.products[p.SKU]; ok {
		return ErrDuplicateSKU
	}
	s.products[p.SKU] = p
	return nil
}

// Get returns the product with the given SKU, or ErrNotFound.
func (s *Store) Get(_ context.Context, sku string) (Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[sku]
	if !ok {
		return Product{}, ErrNotFound
	}
	return p, nil
}

// GetMany returns the products that exist among skus, in no particular
// order. A Postgres store would use WHERE sku = ANY($1).
func (s *Store) GetMany(_ context.Context, skus []string) ([]Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Product, 0, len(skus))
	for _, sku := range skus {
		if p, ok := s.products[sku]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}
