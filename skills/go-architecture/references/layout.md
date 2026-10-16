# Layout by stage

## Contents
- Stage 1: flat (CLI and small service)
- Stage 2: feature packages
- Stage 3: enforced boundaries
- Inside a feature package
- Naming
- Top-level directories

## Stage 1: flat

A CLI or tool:

```
photorename/
  go.mod
  main.go          // flag parsing, calls run(ctx, args, stdout, stderr) error
  rename.go        // the actual work: pure functions where possible
  exif.go          // reading metadata
  rename_test.go
  testdata/        // sample inputs for tests
```

A small service:

```
pinger/
  go.mod
  main.go          // config, wiring, run(ctx, getenv, stdout) error
  server.go        // handlers
  store.go         // data access, concrete type
  store_test.go
```

`main()` only sets up signal handling and calls `run`, which returns an
error. That keeps the program testable with one exit path.

A CLI with several subcommands can stay flat with one file per command
(`cmd_rename.go`, `cmd_scan.go`). Reach for a CLI framework (cobra) only
when there are many subcommands with shared flags.

Move to Stage 2 when a trigger in SKILL.md fires. Moving is mechanical:
create `cmd/<name>/main.go`, move each concern into `internal/<domain>/`.

## Stage 2: feature packages

```
shop/
  go.mod
  cmd/
    api/main.go                 // config, connections, constructors, servers
    worker/main.go              // only if a second binary exists
  internal/
    order/                      // one domain = one package
      order.go                  // domain types, invariants, domain errors
      service.go                // Service: business operations
      store.go                  // Store: SQL for this domain's tables
      http.go                   // Handler: routes, wire types, error mapping
      service_test.go
      http_test.go
    catalog/
      ...
    platform/                   // infrastructure, no business rules
      postgres/                 // pool, DBTX, InTx
      stripe/                   // adapters for external systems
      web/                      // JSON, error envelope, middleware, Serve
      config/
  migrations/
  api/openapi.yaml              // if you publish a contract
  .golangci.yml
  Makefile or Taskfile.yml
```

Rules of thumb:

- One deployable can hold many domains. Domains are packages, not
  services. Split deployables for operational reasons (scaling, release
  cadence, team ownership), never for tidiness.
- `platform/` may be imported by any domain and imports none.
- A domain may import another domain's exported API when the dependency
  is real and one-directional; that is the default. A consumer-side
  interface with a self-contained contract is for providers owned by
  another team or expected to move out of process (see `boundaries.md`).

## Stage 3: enforced boundaries

Same tree, plus:

```
internal/
  order/
    order.go                    // public types used by other domains
    service.go                  // public Service API
    store.go                    // wraps the generated queries below
    http.go
    events.go                   // events this domain publishes
    internal/                   // compiler-enforced: only order/... may import
      db/                       // sqlc output: imports nothing from order
      pricing/                  // pure sub-domain logic: takes and returns its own types
  billing/
    ...
cmd/
  api/ worker/ migrate/
docs/
  architecture.md               // domain map: who may depend on whom
  adr/0001-record-architecture-decisions.md
CODEOWNERS                      // one owner per internal/<domain>/
```

How the nested `internal/` works, and its limits:

- Go only lets packages rooted at `internal/order/` import
  `internal/order/internal/...`. `cmd/api` and other domains cannot.
- Therefore code under `order/internal/` must be **leaf code that never
  imports `order`** (that would be a cycle): generated queries, pure
  algorithms on their own types. The `order` package wraps it, and
  `main` only ever calls `order.NewStore(db)`, `order.NewService(...)`,
  `order.NewHandler(...)`.
- Don't move `Store` or `Service` themselves under `internal/`; they
  must return `order.Order`, and `main` must construct them.

Also at Stage 3:

- Import rules in CI derived from the real dependency graph
  (`boundaries.md` has a working depguard config).
- Cross-domain side effects that needn't be synchronous move to events
  with an outbox.
- A multi-module repo (`go.work`) only when parts need independent
  versioning or release; separate modules add friction to every
  cross-cutting change.

## Inside a feature package

| File | Contains | May import |
|---|---|---|
| `<domain>.go` | Types, invariant-enforcing constructors, domain errors, pure rules | stdlib (no `net/http`, no `database/sql`) |
| `service.go` | `Service`, its dependencies, use cases, what is atomic (via `store.InTx`) | domain, other domains' APIs, consumer interfaces; never `platform/postgres` |
| `store.go` | `Store` over `postgres.DB` or generated queries; `InTx`; row mapping; error translation | `database/sql`, `platform/postgres`, `internal/db` |
| `http.go` | `Handler`, `Routes(mux)`, request/response types, error mapping | `net/http`, `platform/web` |
| `events.go` | Event types the domain publishes (Stage 3) | stdlib |

Split a file past ~500 lines by sub-feature (`service_refund.go`), not
into a new layer. Split a package past ~15-20 files or with unrelated
reasons to change by sub-domain (`order/` into `order/` and
`fulfilment/`).

## Naming

- Package names: short, lower case, singular, no underscores: `order`,
  `catalog`, `postgres`, `web`.
- Exported names read well with the prefix: `order.Service`,
  `order.NewStore`, `catalog.ErrNotFound`, `web.Problem`.
- Constructors: `NewService`, `NewStore`, `NewHandler`; `New` when the
  package has one main type.
- Receivers: one or two letters, consistent per type.
- Interfaces named for behaviour: `PaymentGateway`, `Notifier`, never
  `IOrderRepository`.
- Avoid naming variables after packages you import (`log`, `url`,
  `http`); use `logger`, `u`, `client`.

## Top-level directories

| Directory | Use |
|---|---|
| `cmd/<binary>/` | `main.go` per binary; wiring only |
| `internal/` | All application code |
| `internal/platform/` | Infrastructure shared by domains |
| `migrations/` | Versioned SQL migrations |
| `api/` | OpenAPI or protobuf contracts and generation config |
| `web/` | Templates and static assets for server-rendered apps, usually `embed`ed |
| `docs/` | `architecture.md`, `adr/` |
| `scripts/` | Dev tooling only |

Avoid `pkg/` in applications, `src/`, and a top-level `models/`.
