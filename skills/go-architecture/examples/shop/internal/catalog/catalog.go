// Package catalog owns the products that can be sold and their prices.
package catalog

import (
	"errors"
	"fmt"
	"strings"
)

// Errors callers branch on.
var (
	ErrNotFound       = errors.New("catalog: product not found")
	ErrInvalidProduct = errors.New("catalog: invalid product")
	ErrDuplicateSKU   = errors.New("catalog: duplicate sku")
)

// Product is an item that can be ordered. Prices are integer cents to
// avoid floating-point rounding.
type Product struct {
	SKU        string
	Name       string
	PriceCents int64
}

// Validate reports whether p satisfies the catalog's invariants.
func (p Product) Validate() error {
	switch {
	case strings.TrimSpace(p.SKU) == "":
		return fmt.Errorf("%w: sku is required", ErrInvalidProduct)
	case strings.TrimSpace(p.Name) == "":
		return fmt.Errorf("%w: name is required", ErrInvalidProduct)
	case p.PriceCents <= 0:
		return fmt.Errorf("%w: price must be positive", ErrInvalidProduct)
	}
	return nil
}
