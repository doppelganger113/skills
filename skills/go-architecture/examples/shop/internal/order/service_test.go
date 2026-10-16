package order_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"example.com/shop/internal/catalog"
	"example.com/shop/internal/order"
)

var fixedNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// fakeGateway stands in for the payment provider, the one dependency
// tests can't run for real. It records charges and can be told to fail.
type fakeGateway struct {
	mu      sync.Mutex
	charges map[string]int64 // idempotency key to amount, successful only
	keys    []string         // every key tried, in order
	calls   int
	fail    error
}

func (g *fakeGateway) Charge(_ context.Context, key string, amountCents int64) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	g.keys = append(g.keys, key)
	if g.fail != nil {
		return "", g.fail
	}
	if g.charges == nil {
		g.charges = make(map[string]int64)
	}
	g.charges[key] = amountCents
	return "ch_" + key, nil
}

func (g *fakeGateway) setFail(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fail = err
}

// declinedErr is how a provider signals a refusal under the
// PaymentGateway contract.
type declinedErr struct{}

func (declinedErr) Error() string  { return "card declined" }
func (declinedErr) Declined() bool { return true }

// newTestService builds an order service on real dependencies (the real
// catalog service and real stores) plus the fake gateway. Only time,
// IDs and the payment provider are controlled.
func newTestService(t *testing.T, gw *fakeGateway) *order.Service {
	t.Helper()
	cat := catalog.NewService(catalog.NewStore())
	for _, p := range []catalog.Product{
		{SKU: "TEA", Name: "Green tea", PriceCents: 450},
		{SKU: "MUG", Name: "Mug", PriceCents: 1200},
	} {
		if _, err := cat.Add(t.Context(), p); err != nil {
			t.Fatalf("seed catalog: %v", err)
		}
	}
	var mu sync.Mutex
	n := 0
	newID := func() string {
		mu.Lock()
		defer mu.Unlock()
		n++
		return fmt.Sprintf("ord-%d", n)
	}
	return order.NewService(order.NewStore(), cat, gw, func() time.Time { return fixedNow }, newID)
}

func TestPlace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		lines     []order.LineRequest
		wantTotal int64
		wantErr   error // matched with errors.Is; nil means success
		wantField string
	}{
		{
			name:      "prices lines at catalog price",
			lines:     []order.LineRequest{{SKU: "TEA", Quantity: 2}, {SKU: "MUG", Quantity: 1}},
			wantTotal: 2*450 + 1200,
		},
		{
			name:    "unknown product",
			lines:   []order.LineRequest{{SKU: "NOPE", Quantity: 1}},
			wantErr: order.ErrUnknownProduct,
		},
		{name: "no lines", lines: nil, wantField: "lines"},
		{name: "zero quantity", lines: []order.LineRequest{{SKU: "TEA", Quantity: 0}}, wantField: "lines[0].quantity"},
		{name: "duplicate sku", lines: []order.LineRequest{{SKU: "TEA", Quantity: 1}, {SKU: "TEA", Quantity: 1}}, wantField: "lines[1].sku"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gw := &fakeGateway{}
			got, err := newTestService(t, gw).Place(t.Context(), "", tt.lines)

			if tt.wantField != "" {
				var verr *order.ValidationError
				if !errors.As(err, &verr) || verr.Field != tt.wantField {
					t.Fatalf("Place() error = %v, want ValidationError on %q", err, tt.wantField)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Place() error = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.TotalCents() != tt.wantTotal {
				t.Errorf("TotalCents() = %d, want %d", got.TotalCents(), tt.wantTotal)
			}
			if got.Status != order.StatusPaid || got.PaymentID == "" {
				t.Errorf("Status, PaymentID = %q, %q; want paid with a payment ID", got.Status, got.PaymentID)
			}
			if key := "order-" + got.ID + "-charge-1"; gw.charges[key] != tt.wantTotal {
				t.Errorf("charged %d under key %q, want %d", gw.charges[key], key, tt.wantTotal)
			}
			if !got.CreatedAt.Equal(fixedNow) {
				t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, fixedNow)
			}
		})
	}
}

func TestGetReturnsPlacedOrder(t *testing.T) {
	t.Parallel()
	svc := newTestService(t, &fakeGateway{})
	ctx := t.Context()
	placed, err := svc.Place(ctx, "", []order.LineRequest{{SKU: "MUG", Quantity: 3}})
	if err != nil {
		t.Fatalf("Place() error = %v", err)
	}

	got, err := svc.Get(ctx, placed.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != placed.ID || got.TotalCents() != 3600 || got.Status != order.StatusPaid {
		t.Errorf("Get() = %+v, want paid order %s totalling 3600", got, placed.ID)
	}

	if _, err := svc.Get(ctx, "missing"); !errors.Is(err, order.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestPlacePaymentFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		fail         error
		wantDeclined bool
	}{
		{"provider outage", errors.New("connection reset"), false},
		{"card declined", declinedErr{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newTestService(t, &fakeGateway{fail: tt.fail})
			ctx := t.Context()

			_, err := svc.Place(ctx, "", []order.LineRequest{{SKU: "TEA", Quantity: 1}})
			var perr *order.PaymentError
			if !errors.As(err, &perr) || perr.OrderID != "ord-1" || perr.Declined != tt.wantDeclined {
				t.Fatalf("Place() error = %v, want PaymentError for ord-1 with Declined=%v", err, tt.wantDeclined)
			}

			got, err := svc.Get(ctx, "ord-1")
			if err != nil {
				t.Fatalf("Get() error = %v; the order should be saved before charging", err)
			}
			if got.Status != order.StatusPending || got.PaymentID != "" {
				t.Errorf("Status, PaymentID = %q, %q; want pending and unpaid", got.Status, got.PaymentID)
			}
		})
	}
}

func TestPayRetriesPendingOrderOnce(t *testing.T) {
	t.Parallel()
	gw := &fakeGateway{fail: errors.New("connection reset")}
	svc := newTestService(t, gw)
	ctx := t.Context()

	if _, err := svc.Place(ctx, "", []order.LineRequest{{SKU: "MUG", Quantity: 1}}); err == nil {
		t.Fatal("Place() succeeded, want payment failure")
	}

	gw.setFail(nil) // provider is back
	paid, err := svc.Pay(ctx, "ord-1")
	if err != nil {
		t.Fatalf("Pay() error = %v", err)
	}
	if paid.Status != order.StatusPaid || gw.charges["order-ord-1-charge-1"] != 1200 {
		t.Fatalf("after Pay: status %q, charges %v; want paid, 1200 under attempt 1", paid.Status, gw.charges)
	}
	// An outage is an unknown outcome, so the retry reused the key.
	if want := []string{"order-ord-1-charge-1", "order-ord-1-charge-1"}; !slices.Equal(gw.keys, want) {
		t.Errorf("keys tried = %v, want %v", gw.keys, want)
	}

	// Paying a paid order is a no-op, not a second charge.
	callsBefore := gw.calls
	if _, err := svc.Pay(ctx, "ord-1"); err != nil {
		t.Fatalf("second Pay() error = %v", err)
	}
	if gw.calls != callsBefore {
		t.Errorf("second Pay() called the provider again")
	}

	if _, err := svc.Pay(ctx, "missing"); !errors.Is(err, order.ErrNotFound) {
		t.Errorf("Pay(missing) error = %v, want ErrNotFound", err)
	}
}

func TestPayAfterDeclineUsesNewKey(t *testing.T) {
	t.Parallel()
	gw := &fakeGateway{fail: declinedErr{}}
	svc := newTestService(t, gw)
	ctx := t.Context()

	if _, err := svc.Place(ctx, "", []order.LineRequest{{SKU: "TEA", Quantity: 1}}); err == nil {
		t.Fatal("Place() succeeded, want decline")
	}
	gw.setFail(nil) // customer fixed their card
	if _, err := svc.Pay(ctx, "ord-1"); err != nil {
		t.Fatalf("Pay() error = %v", err)
	}
	// A definite decline starts a new attempt, so the provider can't
	// replay the old decline for the retry.
	if want := []string{"order-ord-1-charge-1", "order-ord-1-charge-2"}; !slices.Equal(gw.keys, want) {
		t.Errorf("keys tried = %v, want %v", gw.keys, want)
	}
}

func TestPlaceWithRequestKeyIsIdempotent(t *testing.T) {
	t.Parallel()
	gw := &fakeGateway{}
	svc := newTestService(t, gw)
	ctx := t.Context()
	lines := []order.LineRequest{{SKU: "TEA", Quantity: 1}}

	first, err := svc.Place(ctx, "client-key-1", lines)
	if err != nil {
		t.Fatalf("first Place() error = %v", err)
	}
	again, err := svc.Place(ctx, "client-key-1", lines)
	if err != nil {
		t.Fatalf("repeated Place() error = %v", err)
	}
	if again.ID != first.ID || gw.calls != 1 {
		t.Errorf("repeat returned order %s after %d charge calls; want %s and 1 call", again.ID, gw.calls, first.ID)
	}

	reordered := []order.LineRequest{{SKU: "TEA", Quantity: 1}}
	if _, err := svc.Place(ctx, "client-key-1", reordered); err != nil {
		t.Errorf("same request, same key: error = %v, want replay", err)
	}

	different := []order.LineRequest{{SKU: "MUG", Quantity: 2}}
	if _, err := svc.Place(ctx, "client-key-1", different); !errors.Is(err, order.ErrRequestKeyReused) {
		t.Errorf("different request, same key: error = %v, want ErrRequestKeyReused", err)
	}

	other, err := svc.Place(ctx, "client-key-2", lines)
	if err != nil || other.ID == first.ID {
		t.Errorf("new key: order %s, err %v; want a new order", other.ID, err)
	}
}

func TestReplayOfUnpaidOrderReportsAwaitingPayment(t *testing.T) {
	t.Parallel()
	gw := &fakeGateway{fail: errors.New("connection reset")}
	svc := newTestService(t, gw)
	ctx := t.Context()
	lines := []order.LineRequest{{SKU: "TEA", Quantity: 1}}

	if _, err := svc.Place(ctx, "k", lines); err == nil {
		t.Fatal("Place() succeeded, want payment failure")
	}
	callsBefore := gw.calls
	_, err := svc.Place(ctx, "k", lines)
	var aerr *order.AwaitingPaymentError
	if !errors.As(err, &aerr) || aerr.OrderID != "ord-1" {
		t.Fatalf("replay error = %v, want AwaitingPaymentError for ord-1", err)
	}
	if gw.calls != callsBefore {
		t.Errorf("replay called the provider; a replay must not charge")
	}
}
