package order

import (
	"context"
	"slices"
	"sync"
)

// Store persists orders.
//
// In-memory here so the example has no dependencies. A Postgres version
// keeps the same methods and error contract (translating sql.ErrNoRows
// into ErrNotFound) and adds InTx; see references/persistence.md.
type Store struct {
	mu        sync.RWMutex
	orders    map[string]Order
	byRequest map[string]string // request key to order ID
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{orders: make(map[string]Order), byRequest: make(map[string]string)}
}

// Insert saves o. A non-empty RequestKey must be unique (a unique index
// in Postgres); a second insert with the same key returns
// ErrDuplicateRequest.
//
// This example has no authentication, so keys are unique across all
// clients. With authentication, make the key unique per caller (a
// unique index on customer or tenant ID plus key) and look it up only
// within the caller's scope, or one client could replay another's.
func (s *Store) Insert(_ context.Context, o Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o.RequestKey != "" {
		if _, taken := s.byRequest[o.RequestKey]; taken {
			return ErrDuplicateRequest
		}
		s.byRequest[o.RequestKey] = o.ID
	}
	o.Lines = slices.Clone(o.Lines) // callers must not share our slice
	s.orders[o.ID] = o
	return nil
}

// GetByRequestKey returns the order created with key, or ErrNotFound.
func (s *Store) GetByRequestKey(ctx context.Context, key string) (Order, error) {
	s.mu.RLock()
	id, ok := s.byRequest[key]
	s.mu.RUnlock()
	if !ok {
		return Order{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// RecordDecline starts a new payment attempt after a definite decline,
// so the next charge uses a fresh provider idempotency key.
func (s *Store) RecordDecline(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return ErrNotFound
	}
	o.PaymentAttempt++
	s.orders[id] = o
	return nil
}

// MarkPaid records a successful charge, or returns ErrNotFound.
func (s *Store) MarkPaid(_ context.Context, id, paymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return ErrNotFound
	}
	o.Status, o.PaymentID = StatusPaid, paymentID
	s.orders[id] = o
	return nil
}

// Get returns the order with the given ID, or ErrNotFound.
func (s *Store) Get(_ context.Context, id string) (Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}
	o.Lines = slices.Clone(o.Lines)
	return o, nil
}
