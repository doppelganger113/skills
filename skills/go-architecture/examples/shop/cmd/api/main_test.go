package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestEndToEnd drives the fully wired handler: the same graph main
// builds, with real services and stores.
func TestEndToEnd(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newHandler(slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(srv.Close)

	steps := []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/healthz", "", http.StatusNoContent},
		{"POST", "/products", `{"sku":"TEA","name":"Green tea","price_cents":450}`, http.StatusCreated},
		{"POST", "/orders", `{"lines":[{"sku":"TEA","quantity":2}]}`, http.StatusCreated},
	}
	for _, s := range steps {
		req, err := http.NewRequestWithContext(t.Context(), s.method, srv.URL+s.path, strings.NewReader(s.body))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != s.want {
			t.Fatalf("%s %s status = %d, want %d", s.method, s.path, resp.StatusCode, s.want)
		}
		if resp.Header.Get("X-Request-ID") == "" {
			t.Errorf("%s %s: missing X-Request-ID header", s.method, s.path)
		}
	}
}

func TestRunShutsDownOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // run must bind, start, then shut down cleanly

	env := map[string]string{"SHOP_ADDR": "127.0.0.1:0"}
	done := make(chan error, 1)
	go func() { done <- run(ctx, func(k string) string { return env[k] }, io.Discard) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() = %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return after cancellation")
	}
}
