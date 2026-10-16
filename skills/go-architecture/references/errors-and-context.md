# Errors, logging, context and lifecycle

## Contents
- Error design
- One error envelope, per-domain mapping
- HTTP API conventions
- Logging and request IDs
- Middleware
- Context
- Lifecycle and graceful shutdown
- Concurrency ownership
- Observability and auth at scale

## Error design

- Domains define only the errors callers branch on:

```go
var (
	ErrNotFound       = errors.New("order: not found")
	ErrUnknownProduct = errors.New("order: unknown product")
)

// ValidationError reports which part of a request is invalid.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("order: invalid %s: %s", e.Field, e.Reason)
}
```

- Sentinels for conditions; a struct type when callers need data.
- Wrap at each layer that adds information:
  `fmt.Errorf("get order %s: %w", id, err)`. Say what you were doing,
  not "failed to".
- Translate infrastructure errors at the store (`persistence.md`).
- Handle each error once: return it wrapped, or log it and stop.
- `panic` only for programmer errors at start-up. Recovery middleware is
  a safety net, not error handling.

## One error envelope, per-domain mapping

Clients should see one error format across every domain. `platform/web`
owns the envelope (RFC 9457 problem details plus a stable `code`);
each domain's handler maps only its own errors and falls through to
the shared internal-error response:

```go
// platform/web
func Problem(w http.ResponseWriter, status int, code, detail string)
func InternalError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error)
func Decode(w http.ResponseWriter, r *http.Request, v any) bool // writes 400 or 413 itself

// order/http.go
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var verr *ValidationError
	switch {
	case errors.As(err, &verr):
		web.Problem(w, http.StatusUnprocessableEntity, "order.invalid", verr.Error())
	case errors.Is(err, ErrNotFound):
		web.Problem(w, http.StatusNotFound, "order.not_found", "order not found")
	default:
		web.InternalError(w, r, h.logger, err) // logs; reveals nothing
	}
}
```

Codes are namespaced by domain (`order.not_found`) and never change once
published; `detail` is human text and may.

## HTTP API conventions

House defaults, applied when the repo hasn't chosen otherwise (a
repo's `CLAUDE.md` or existing API wins). What matters is one set for
every domain, so thirty teams don't invent thirty:

- **Lists are always paginated.** Query parameters `limit` (default 50,
  max 200) and `cursor` (opaque, keyset-based: encode the last row's
  sort key; never an offset). Response:
  `{"items": [...], "next_cursor": "..."}`, empty `next_cursor` on the
  last page. `Page{Limit int; After string}` lives in a tiny
  `internal/platform/page` package with no imports (never in `web`, or
  every store would import HTTP code). The handler parses `limit` and
  `cursor` into a `Page`; the store encodes and decodes the keyset
  cursor and returns the next one:
  `ListByCustomer(ctx, customerID string, p page.Page) ([]Invoice, string, error)`.
- **Nested routes belong to the owner of the returned data.**
  `GET /customers/{id}/invoices` is registered by `invoice`. For an
  unknown customer, return an empty list by default; return 404 only if
  the product needs it, by checking through a consumer interface
  (`CustomerExists(ctx, id) (bool, error)`), never by querying the
  customer tables.
- **Wire types are separate from domain types** once the API is
  consumed by anyone else; JSON field names in `snake_case`; money as
  integer minor units with a currency field when there is more than one
  currency; timestamps as RFC 3339 UTC.
- **Creation returns 201 with a `Location` header.** Validation errors
  are 422 with the field named; malformed bodies 400; oversized 413.
- **Idempotency** for POSTs that move money, from day one (the
  example's `POST /orders`); for other POSTs that send or publish
  something, once a client actually retries them. Plain creates need
  none of this:
  - Accept an `Idempotency-Key` header and store it with a fingerprint
    of the request on the created record.
  - Scope keys to the caller: unique on (customer or tenant ID, key),
    looked up only within the caller's scope, never globally.
  - A replay must match the original request and report the original
    outcome: same key and body replays the stored result; same key with
    a different body is rejected (422); an original that is still
    awaiting payment is reported as such (409 with its `Location`), not
    as a fresh 201.

## Logging and request IDs

- `log/slog`, created in `main`, injected into components that log. No
  package-level loggers. Don't name the variable `log`.
- Stable keys (`order_id` everywhere, not `orderID` here and `id`
  there).
- **The standard handlers ignore the context.** `logger.InfoContext(ctx,
  ...)` only adds request fields if the handler reads them, so wrap it:

```go
type ContextHandler struct{ slog.Handler }

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}
// plus WithAttrs and WithGroup returning ContextHandler{...}
```

- Always pass the context: `logger.ErrorContext(r.Context(), ...)`.
- Services rarely log; they return errors. Transport and workers decide
  what is worth logging.

## Middleware

Plain `func(http.Handler) http.Handler` style in `platform/web`, applied
in `main`, outermost first:

1. Request ID (read `X-Request-ID` or generate; store in context; echo).
2. Request logging (method, path, status, duration).
3. Panic recovery (log with stack, 500; re-panic
   `http.ErrAbortHandler`).
4. At Stage 3: tracing, metrics, authentication.

A response-writer wrapper must implement `Unwrap() http.ResponseWriter`
so `http.ResponseController` can still flush and set deadlines.

## Context

- First parameter, named `ctx`, on anything that does I/O or blocks.
- Never stored in a struct; never `nil`. Use `t.Context()` in tests.
- Context values only for request-scoped data crossing API boundaries
  (request ID, trace, authenticated principal), with unexported key
  types and typed accessors. Never for dependencies or optional
  parameters.

## Lifecycle and graceful shutdown

```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

The server runs until `ctx` is done, then `Shutdown` with a timeout on a
`context.WithoutCancel` context. Set `ReadHeaderTimeout`, `ReadTimeout`,
`WriteTimeout` and `IdleTimeout` on every `http.Server`; zero means no
timeout. `Serve` takes a `net.Listener`, so callers and tests can bind
first.

## Concurrency ownership

- Whoever starts a goroutine owns it: it can stop it and waits for it.
  `errgroup.WithContext` for work that should fail together, for
  example the HTTP server and a background relay in one binary.
- No fire-and-forget `go f()` in handlers; work that outlives the
  request goes to a queue or worker with its own lifecycle.
- A mutex sits next to the data it guards. Channels for real pipelines,
  not as a lock.
- `go test -race` in CI.

## Observability and auth at scale

- **Tracing and metrics (Stage 3):** OpenTelemetry, set up in
  `platform/telemetry` and started in `main`. Instrument at the edges:
  `otelhttp` around the mux, an instrumented database driver, and
  propagation on outgoing HTTP clients. Domain code only passes `ctx`.
- **Health:** `/healthz` (process alive) and `/readyz` (dependencies
  reachable) registered in `main`.
- **Authentication:** middleware verifies the credential and stores a
  small `auth.Principal` (user ID, tenant ID, roles) in the context via
  a typed accessor in `platform/auth`. **Authorisation** is a business
  rule: services check it against the principal and return a domain
  `ErrForbidden`, so it is tested with the rest of the logic.
