package catalog

import (
	"context"
	"fmt"
)

// Service implements the catalog's business operations.
type Service struct {
	store *Store
}

// NewService returns a Service backed by store.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// Add validates p and stores it.
func (s *Service) Add(ctx context.Context, p Product) (Product, error) {
	if err := p.Validate(); err != nil {
		return Product{}, err
	}
	if err := s.store.Insert(ctx, p); err != nil {
		return Product{}, fmt.Errorf("add product %s: %w", p.SKU, err)
	}
	return p, nil
}

// Get returns the product with the given SKU.
func (s *Service) Get(ctx context.Context, sku string) (Product, error) {
	p, err := s.store.Get(ctx, sku)
	if err != nil {
		return Product{}, fmt.Errorf("get product %s: %w", sku, err)
	}
	return p, nil
}

// Prices returns the current unit price in cents of each known SKU in
// one call. Unknown SKUs are absent from the result rather than an
// error, so callers decide what "unknown" means for them. One batched
// call per request keeps this cheap even if the catalog later sits
// behind a network hop.
func (s *Service) Prices(ctx context.Context, skus []string) (map[string]int64, error) {
	products, err := s.store.GetMany(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("get prices: %w", err)
	}
	prices := make(map[string]int64, len(products))
	for _, p := range products {
		prices[p.SKU] = p.PriceCents
	}
	return prices, nil
}
