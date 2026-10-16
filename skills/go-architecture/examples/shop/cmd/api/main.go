// Command api serves the shop HTTP API.
//
// main is the only place that knows every component: it reads config,
// constructs each dependency with plain constructors, wires them
// together and owns the process lifecycle.
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/shop/internal/catalog"
	"example.com/shop/internal/order"
	"example.com/shop/internal/platform/payments"
	"example.com/shop/internal/platform/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// config is parsed once here and passed down as typed values; no other
// package reads the environment. A larger service moves this to
// internal/platform/config.
type config struct {
	Addr string
}

func loadConfig(getenv func(string) string) config {
	cfg := config{Addr: getenv("SHOP_ADDR")}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	return cfg
}

// run is main without the process exits, so it can be tested.
func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	cfg := loadConfig(getenv)
	logger := slog.New(web.ContextHandler{Handler: slog.NewJSONHandler(stdout, nil)})

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Addr, err)
	}
	return web.Serve(ctx, ln, newHandler(logger), logger)
}

// newHandler builds the whole object graph and returns the root HTTP
// handler. In a real service, connections opened in run (database,
// queues) are passed in here as well.
func newHandler(logger *slog.Logger) http.Handler {
	catalogSvc := catalog.NewService(catalog.NewStore())
	orderSvc := order.NewService(order.NewStore(), catalogSvc, payments.NewSandbox(), time.Now, rand.Text)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	catalog.NewHandler(catalogSvc, logger).Routes(mux)
	order.NewHandler(orderSvc, logger).Routes(mux)

	// Outermost first: every request gets an ID, panics become 500s, and
	// the log line carries the final status.
	return web.WithRequestID(web.LogRequests(web.Recover(mux, logger), logger))
}
