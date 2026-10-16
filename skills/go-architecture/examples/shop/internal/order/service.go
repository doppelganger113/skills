package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example.com/shop/internal/catalog"
)

// PaymentGateway charges customers. It is an interface because the real
// provider can't run in tests (trigger: dependency you can't run); it is
// declared here, by the consumer, with only the method order calls.
//
// Contract: charging twice with the same idempotencyKey returns the
// original result instead of charging again. A refused payment returns
// an error with a Declined() bool method reporting true, so order never
// imports the provider's error types.
type PaymentGateway interface {
	Charge(ctx context.Context, idempotencyKey string, amountCents int64) (chargeID string, err error)
}

// Service implements order business operations.
//
// It calls the catalog directly: importing another domain's exported
// API is the default. Only the payment provider sits behind an
// interface.
type Service struct {
	store    *Store
	catalog  *catalog.Service
	payments PaymentGateway
	now      func() time.Time
	newID    func() string
}

// NewService returns a Service. now and newID are injected so tests are
// deterministic; main passes time.Now and a real ID generator.
func NewService(store *Store, cat *catalog.Service, payments PaymentGateway, now func() time.Time, newID func() string) *Service {
	return &Service{store: store, catalog: cat, payments: payments, now: now, newID: newID}
}

// Place prices each line at the current catalog price, saves the order
// as pending, then charges it. If the charge fails, the returned
// *PaymentError names the saved order so the caller can retry with Pay.
//
// requestKey is the client's Idempotency-Key, or "" if none was sent. A
// repeated key with the same request replays the original outcome and
// charges nothing, so a client retrying after a timeout never creates a
// second order or a second charge. The same key with a different
// request returns ErrRequestKeyReused; a replay of an order that was
// never paid returns *AwaitingPaymentError.
func (s *Service) Place(ctx context.Context, requestKey string, lines []LineRequest) (Order, error) {
	hash := requestHash(lines)
	if requestKey != "" {
		o, err := s.store.GetByRequestKey(ctx, requestKey)
		if err == nil {
			return replay(o, hash)
		}
		if !errors.Is(err, ErrNotFound) {
			return Order{}, fmt.Errorf("look up request key: %w", err)
		}
	}
	if err := validateLines(lines); err != nil {
		return Order{}, err
	}

	skus := make([]string, len(lines))
	for i, l := range lines {
		skus[i] = l.SKU
	}
	prices, err := s.catalog.Prices(ctx, skus)
	if err != nil {
		return Order{}, fmt.Errorf("look up prices: %w", err)
	}

	o := Order{
		ID:             s.newID(),
		RequestKey:     requestKey,
		RequestHash:    hash,
		Status:         StatusPending,
		PaymentAttempt: 1,
		CreatedAt:      s.now().UTC(),
		Lines:          make([]Line, len(lines)),
	}
	for i, l := range lines {
		cents, ok := prices[l.SKU]
		if !ok {
			return Order{}, fmt.Errorf("%w: %s", ErrUnknownProduct, l.SKU)
		}
		o.Lines[i] = Line{SKU: l.SKU, Quantity: l.Quantity, UnitPriceCents: cents}
	}

	// Save first, charge second: a failed charge leaves a pending order
	// that can be retried, never a charge with no order.
	err = s.store.Insert(ctx, o)
	if errors.Is(err, ErrDuplicateRequest) {
		// A concurrent request with the same key won the race.
		first, err := s.store.GetByRequestKey(ctx, requestKey)
		if err != nil {
			return Order{}, fmt.Errorf("look up request key: %w", err)
		}
		return replay(first, hash)
	}
	if err != nil {
		return Order{}, fmt.Errorf("save order: %w", err)
	}
	return s.charge(ctx, o)
}

// replay answers a repeated Idempotency-Key with the original outcome.
// A different request under the same key is an error, not a replay, and
// an order that was never paid is reported as such rather than as a
// fresh success.
func replay(o Order, hash string) (Order, error) {
	switch {
	case o.RequestHash != hash:
		return Order{}, ErrRequestKeyReused
	case o.Status == StatusPending:
		return Order{}, &AwaitingPaymentError{OrderID: o.ID}
	}
	return o, nil
}

// Pay charges a pending order. It is safe to call repeatedly, from a
// client retry or a reconciliation job: a paid order is returned as is,
// and the attempt's idempotency key means the provider never charges
// twice, even if an earlier attempt charged but failed to record it.
func (s *Service) Pay(ctx context.Context, id string) (Order, error) {
	o, err := s.Get(ctx, id)
	if err != nil {
		return Order{}, err
	}
	if o.Status == StatusPaid {
		return o, nil
	}
	return s.charge(ctx, o)
}

// charge calls the provider and records the result. It runs outside any
// transaction on purpose: holding a database transaction open across a
// network call ties up a connection, and a rollback can't undo a charge.
//
// An unknown outcome (timeout, outage) keeps the attempt, so a retry
// reuses the key and the provider deduplicates. A definite decline
// starts a new attempt, so a retry after the customer fixes their card
// gets a fresh key instead of the provider replaying the decline.
func (s *Service) charge(ctx context.Context, o Order) (Order, error) {
	paymentID, err := s.payments.Charge(ctx, o.chargeKey(), o.TotalCents())
	if err != nil {
		var d interface{ Declined() bool }
		declined := errors.As(err, &d) && d.Declined()
		if declined {
			if rerr := s.store.RecordDecline(ctx, o.ID); rerr != nil {
				return Order{}, fmt.Errorf("record decline for order %s: %w", o.ID, rerr)
			}
		}
		return Order{}, &PaymentError{OrderID: o.ID, Declined: declined, cause: err}
	}
	if err := s.store.MarkPaid(ctx, o.ID, paymentID); err != nil {
		// The provider has charged; Pay will record it on retry.
		return Order{}, fmt.Errorf("record payment for order %s: %w", o.ID, err)
	}
	o.Status, o.PaymentID = StatusPaid, paymentID
	return o, nil
}

// Get returns the order with the given ID.
func (s *Service) Get(ctx context.Context, id string) (Order, error) {
	o, err := s.store.Get(ctx, id)
	if err != nil {
		return Order{}, fmt.Errorf("get order %s: %w", id, err)
	}
	return o, nil
}
