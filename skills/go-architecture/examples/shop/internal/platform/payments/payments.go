// Package payments is the adapter for the external payment provider.
//
// Adapters for external systems always live under internal/platform/,
// never in a domain package. A real provider gets a package named for it
// (platform/stripe); this sandbox is named for its role because it
// stands in for whichever provider you choose. This one uses only primitive types, so it
// imports no domain; order.PaymentGateway is satisfied implicitly.
package payments

import (
	"context"
	"errors"
)

// Sandbox stands in for a real provider client (Stripe, Adyen) so the
// example runs without credentials or network. A real client makes the
// HTTP call here and maps provider errors to plain Go errors.
type Sandbox struct{}

// NewSandbox returns a Sandbox client.
func NewSandbox() *Sandbox { return &Sandbox{} }

// sandboxLimitCents is the amount above which the sandbox declines, so
// the decline path can be tried by hand.
const sandboxLimitCents = 1_000_000

// DeclinedError is returned when the provider refuses a payment.
type DeclinedError struct {
	Reason string
}

func (e *DeclinedError) Error() string { return "payments: declined: " + e.Reason }

// Declined reports true. Consumers check for this method instead of
// importing this package's types.
func (e *DeclinedError) Declined() bool { return true }

// Charge pretends to charge amountCents. Charging twice with the same
// idempotencyKey returns the same charge ID, as real providers do.
func (*Sandbox) Charge(_ context.Context, idempotencyKey string, amountCents int64) (string, error) {
	switch {
	case amountCents <= 0:
		return "", errors.New("payments: amount must be positive")
	case amountCents > sandboxLimitCents:
		return "", &DeclinedError{Reason: "over sandbox limit"}
	}
	return "ch_" + idempotencyKey, nil
}
