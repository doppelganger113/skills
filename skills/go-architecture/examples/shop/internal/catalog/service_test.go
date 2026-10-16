package catalog_test

import (
	"errors"
	"maps"
	"testing"

	"example.com/shop/internal/catalog"
)

func TestAdd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		product catalog.Product
		wantErr error
	}{
		{"valid", catalog.Product{SKU: "TEA", Name: "Green tea", PriceCents: 450}, nil},
		{"missing sku", catalog.Product{Name: "Green tea", PriceCents: 450}, catalog.ErrInvalidProduct},
		{"missing name", catalog.Product{SKU: "TEA", PriceCents: 450}, catalog.ErrInvalidProduct},
		{"zero price", catalog.Product{SKU: "TEA", Name: "Green tea"}, catalog.ErrInvalidProduct},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := catalog.NewService(catalog.NewStore())
			_, err := svc.Add(t.Context(), tt.product)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Add() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddRejectsDuplicateSKU(t *testing.T) {
	t.Parallel()
	svc := catalog.NewService(catalog.NewStore())
	p := catalog.Product{SKU: "TEA", Name: "Green tea", PriceCents: 450}
	if _, err := svc.Add(t.Context(), p); err != nil {
		t.Fatalf("first Add() error = %v", err)
	}
	if _, err := svc.Add(t.Context(), p); !errors.Is(err, catalog.ErrDuplicateSKU) {
		t.Errorf("second Add() error = %v, want ErrDuplicateSKU", err)
	}
}

func TestPricesOmitsUnknownSKUs(t *testing.T) {
	t.Parallel()
	svc := catalog.NewService(catalog.NewStore())
	if _, err := svc.Add(t.Context(), catalog.Product{SKU: "TEA", Name: "Green tea", PriceCents: 450}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Prices(t.Context(), []string{"TEA", "NOPE"})
	if err != nil {
		t.Fatalf("Prices() error = %v", err)
	}
	want := map[string]int64{"TEA": 450}
	if !maps.Equal(got, want) {
		t.Errorf("Prices() = %v, want %v", got, want)
	}
}
