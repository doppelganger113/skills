// Package order owns placing and reading customer orders.
package order

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Errors callers branch on.
var (
	ErrNotFound         = errors.New("order: not found")
	ErrUnknownProduct   = errors.New("order: unknown product")
	ErrDuplicateRequest = errors.New("order: request key already used")
	ErrRequestKeyReused = errors.New("order: idempotency key reused with a different request")
)

// AwaitingPaymentError reports a replayed request whose order was saved
// but never paid: the client must pay it with Service.Pay, not assume
// it was created and charged.
type AwaitingPaymentError struct {
	OrderID string
}

func (e *AwaitingPaymentError) Error() string {
	return "order: " + e.OrderID + " is awaiting payment"
}

// PaymentError reports a charge that didn't go through. The order is
// saved and stays pending, so the caller can retry with Service.Pay.
type PaymentError struct {
	OrderID  string
	Declined bool // the provider refused the payment; retrying won't help
	cause    error
}

func (e *PaymentError) Error() string {
	if e.Declined {
		return fmt.Sprintf("order: payment for %s declined: %v", e.OrderID, e.cause)
	}
	return fmt.Sprintf("order: payment for %s failed: %v", e.OrderID, e.cause)
}

func (e *PaymentError) Unwrap() error { return e.cause }

// ValidationError reports which part of an order request is invalid.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("order: invalid %s: %s", e.Field, e.Reason)
}

// LineRequest is one requested product and quantity.
type LineRequest struct {
	SKU      string
	Quantity int
}

// Line is a priced order line. The unit price is captured when the order
// is placed, so later catalog price changes don't alter past orders.
type Line struct {
	SKU            string
	Quantity       int
	UnitPriceCents int64
}

// Status is where an order is in its lifecycle.
type Status string

// Order statuses.
const (
	StatusPending Status = "pending" // saved, not yet charged
	StatusPaid    Status = "paid"
)

// Order is a placed order.
type Order struct {
	ID          string
	RequestKey  string // client's Idempotency-Key; unique when set
	RequestHash string // fingerprint of the request that created the order
	Status      Status
	Lines       []Line
	PaymentID   string // set once paid
	// PaymentAttempt numbers charge attempts. It changes only after a
	// definite decline, so a retry after an unknown outcome (timeout,
	// outage) reuses the same provider idempotency key.
	PaymentAttempt int
	CreatedAt      time.Time
}

// requestHash fingerprints an order request so a replayed
// Idempotency-Key can be checked against the request it was first used
// with. Line order doesn't matter; SKUs and quantities do.
func requestHash(lines []LineRequest) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = fmt.Sprintf("%s\x00%d", l.SKU, l.Quantity)
	}
	slices.Sort(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// chargeKey is the provider idempotency key for the current attempt. A
// key names one attempt of one operation; a refund gets its own key
// (refund-<refundID>) and never reuses a charge's.
func (o Order) chargeKey() string {
	return fmt.Sprintf("order-%s-charge-%d", o.ID, o.PaymentAttempt)
}

// TotalCents returns the sum of all line totals.
func (o Order) TotalCents() int64 {
	var total int64
	for _, l := range o.Lines {
		total += l.UnitPriceCents * int64(l.Quantity)
	}
	return total
}

// maxQuantity bounds a single line to keep totals far from overflow and
// to catch obvious input mistakes.
const maxQuantity = 1000

func validateLines(lines []LineRequest) error {
	if len(lines) == 0 {
		return &ValidationError{Field: "lines", Reason: "at least one line is required"}
	}
	seen := make(map[string]bool, len(lines))
	for i, l := range lines {
		field := fmt.Sprintf("lines[%d]", i)
		switch {
		case l.SKU == "":
			return &ValidationError{Field: field + ".sku", Reason: "is required"}
		case l.Quantity < 1 || l.Quantity > maxQuantity:
			return &ValidationError{Field: field + ".quantity", Reason: fmt.Sprintf("must be between 1 and %d", maxQuantity)}
		case seen[l.SKU]:
			return &ValidationError{Field: field + ".sku", Reason: "appears more than once"}
		}
		seen[l.SKU] = true
	}
	return nil
}
