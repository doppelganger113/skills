# Boundaries, interfaces and wiring

## Contents
- Where adapters for external systems live
- When to introduce an interface
- Designing a consumer interface
- Wiring in main
- Cross-domain calls
- Breaking import cycles
- Transactions across domains
- Events and the outbox
- Enforcing boundaries with depguard

## Where adapters for external systems live

A client for a third-party system (payments, email, another company's
API) is an adapter, not business logic. It always lives in its own
package under `internal/platform/`, named for the provider
(`platform/stripe`, `platform/postmark`), even when only one domain uses
it, so there is one place for adapters and the lint rules need no
exceptions. The vendor SDK is imported only there; the strict lint rule
below rejects it anywhere in a domain's logic files.

Its methods use its own or primitive types, so it never imports a
domain. It satisfies the consumer's interface (`order.PaymentGateway`)
implicitly and is wired in `main`. If the consumer's interface needs
domain types, `main` bridges with a few-line adapter func. It never goes
in a domain file, where the purity rule would rightly reject its
`net/http` import.

## When to introduce an interface

Only at a boundary where one of these holds:

1. There are, or are about to be, two real implementations (`DBTX` over
   `*sql.DB` and `*sql.Tx`).
2. The dependency can't run in tests: third-party APIs, payments, email,
   the clock or randomness when determinism matters.
3. The dependency is another domain owned by a different team, or
   expected to move out of process. Otherwise, call the other domain's
   exported `*Service` directly: that is the default.

Declare it in the consumer's package, next to the code that uses it,
with only the methods the consumer calls. Don't put an interface in
front of your own store to mock it; test against the real store. A
dependency that is one call (the clock, an ID or random source) is a
func parameter, `func() time.Time`, not a one-method interface.

## Designing a consumer interface

An interface only decouples if its contract is self-contained. Check:

- **No imports of the provider.** If the consumer still imports the
  provider to check its errors or types, the interface is decoration.
  Express outcomes in the contract itself (absent from a map, a boolean,
  a returned ID, or a consumer-defined error the adapter returns).
- **Batch-shaped.** Ask for everything a use case needs in one call. A
  per-item method becomes N network calls.
- **Idempotent when the provider is remote.** Pass a key the provider
  can deduplicate on, so a retry never charges or sends twice. A key
  names one attempt of one operation (`order-<id>-charge-<n>`,
  `refund-<refundID>`). Reuse it only when the outcome is unknown
  (timeout, 5xx); after a definite decline, record a new attempt
  before calling again; never share a key between operations.
- **Outcomes without provider types.** Signal outcomes the consumer
  branches on through behaviour, not imports: the example's gateway
  returns an error with a `Declined() bool` method, which `order`
  checks with `errors.As` against an anonymous interface.
- **Consumer vocabulary.** Name it for what the consumer needs
  (`PaymentGateway`), not for the vendor (`StripeClient`).

```go
// package order

// PaymentGateway charges customers. It is an interface because the real
// provider can't run in tests. idempotencyKey makes retries safe.
type PaymentGateway interface {
	Charge(ctx context.Context, idempotencyKey string, amountCents int64) (chargeID string, err error)
}
```

`*stripe.Client` in `platform/stripe` satisfies it implicitly; tests use
a hand-written fake. The same rules apply to the rarer cross-domain
interface (trigger 3): if the other domain's natural method doesn't
match, write a few-line adapter in `main` rather than bending its API
around one consumer.

## Wiring in main

`main` builds the object graph by hand. It is allowed to be long and
boring.

```go
func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := slog.New(web.ContextHandler{Handler: slog.NewJSONHandler(stdout, nil)})

	pool, err := sql.Open("pgx", cfg.DatabaseURL) // needs: import _ "github.com/jackc/pgx/v5/stdlib"
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = pool.Close() }()
	db := postgres.New(pool)

	catalogSvc := catalog.NewService(catalog.NewStore(db))
	payments := stripe.New(cfg.StripeKey)
	orderSvc := order.NewService(order.NewStore(db), catalogSvc, payments, time.Now, rand.Text)

	mux := http.NewServeMux()
	catalog.NewHandler(catalogSvc, logger).Routes(mux)
	order.NewHandler(orderSvc, logger).Routes(mux)
	handler := web.WithRequestID(web.LogRequests(web.Recover(mux, logger), logger))

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Addr, err)
	}
	return web.Serve(ctx, ln, handler, logger)
}
```

Rules:

- No DI containers. At Stage 3, if construction passes a few hundred
  lines, group it into per-domain `wire(...)` functions in `main`, or use
  `google/wire` (compile-time); never runtime reflection containers.
- Config is parsed once into typed structs and passed down; packages
  never call `os.Getenv`.
- `run` takes `getenv`, `stdout` and args so it is testable.

## Cross-domain calls

Default: import the other domain's exported API. Use a consumer
interface only under trigger 3 above.

| Situation | Mechanism |
|---|---|
| A needs B synchronously; B doesn't need A | A calls B's exported API, or a consumer interface B satisfies |
| A and B both need a concept (money, address) | A small package for that concept that both import, named for it (`money`), never `common` |
| B must react to something in A, not necessarily in the same request | A publishes an event; B subscribes |
| A and B change together in almost every PR | They are one domain; merge them |

Domains reference each other by ID. `invoice.Invoice` holds
`CustomerID string`; when a response needs the customer's name, the
service fetches it through `customer`'s API (batched) and composes the
response.

**Read screens that span many domains** (dashboards, reports, admin
lists) don't belong to any one domain. Give them their own read-only
package (`internal/reporting`) that composes batched calls to each
domain. When that becomes too slow, the next step is a read model: a
denormalised table owned by `reporting`, fed by the domains' events.
Never let it read other domains' tables directly.

## Breaking import cycles

A cycle means two packages each own part of one concept. In order of
preference:

1. Replace embedded structs from the other domain with IDs.
2. Move the shared type to the package that truly owns it.
3. Extract a small package for a genuinely shared concept.
4. Invert one edge: the lower package declares an interface or
   callback; `main` connects them.
5. Replace the back-edge with an event.

Never fix a cycle with a `types` or `common` package holding both sides'
structs.

## Transactions across domains

See `persistence.md` for the `DBTX`/`InTx` mechanics. Default to one
domain per transaction. When two domains must change together:

- **Prefer eventual consistency:** domain A commits its change plus an
  outbox row; B reacts.
- **If it truly must be atomic** (Stage 2, one database), the
  coordinating domain's store exposes one named method that binds both
  stores to the same transaction (`persistence.md`, "Across domains").
  At Stage 3, needing this is a sign the boundary is in the wrong place.

## Events and the outbox

- **Stage 2:** an in-process publisher is enough: a slice of
  `func(ctx context.Context, e Event) error` registered in `main`, called
  after commit. Acceptable when losing an event on crash is tolerable.
- **Stage 3, or when events must not be lost:** A writes its change and
  an `outbox` row in the same transaction; a relay in `cmd/worker`
  publishes rows and marks them sent; consumers are idempotent, keyed by
  event ID.
- Event types live in the publishing domain's `events.go` and carry IDs
  plus the facts that changed, not whole aggregates.

## Enforcing boundaries with depguard

Go's nested `internal/` handles "only my domain may import this" for
free. depguard handles "business logic files import no transport or
SQL" and "platform imports no domain". The example's `arch_test.go`
checks what depguard can't express (package names, stutter, globals,
cross-domain struct fields, nested domains); copy it into the repo. The rules are file
scoped because one domain package mixes logic files with adapter files.

The rules rely on two conventions. Inside a domain package, adapter
files are named `http*.go` and `store*.go`; everything else is business
logic. And domains are siblings directly under `internal/`, never
nested inside each other (split `order/` into `order/` and
`fulfilment/`, not `order/fulfilment/`). That way a new domain is
covered with no config change.

Config for golangci-lint v2 (tested against v2.15; the same file is in
`examples/shop/.golangci.yml`):

```yaml
version: "2"
linters:
  enable:
    - depguard
  settings:
    depguard:
      rules:
        business-logic-is-pure:
          list-mode: strict              # only allowed imports pass
          files:
            - "**/internal/*/*.go"
            - "!**/internal/*/http*.go"
            - "!**/internal/*/store*.go"
            - "!**/internal/*/internal/db/**"   # generated queries (Stage 3)
            - "!**/internal/platform/**"
            - "!$test"
          allow:
            - $gostd
            - example.com/shop/internal  # add pure libraries (uuid, decimal) by name
          deny:
            - pkg: net/http
              desc: transport belongs in http.go
            - pkg: database/sql
              desc: persistence belongs in store.go
            - pkg: github.com/jackc/pgx
              desc: persistence belongs in store.go
            - pkg: example.com/shop/internal/platform/postgres
              desc: only store files touch the database
        platform-imports-no-domain:
          list-mode: lax
          files:
            - "**/internal/platform/**"
          allow:
            - example.com/shop/internal/platform   # longer match wins
          deny:
            - pkg: example.com/shop/internal
              desc: platform must not depend on a domain
```

Replace `example.com/shop` with the module path. For a Stage 3 domain
dependency map ("billing may import order, never the reverse"), add one
rule per domain with `files: ["**/internal/order/**"]` denying the
domains it must not use, or use go-arch-lint, which expresses the same
map as a whitelist per component.

Keep the rules few and derived from the real graph. A rule that blocks
a legitimate change is updated in the same PR, with the reason in the
commit message.
