---
name: go-architecture
description: Architecture and design rules for Go code that stays clean as it grows from a small tool to a large multi-team codebase. Use when creating any Go program (service, API, worker or CLI), adding a feature or endpoint, deciding where code goes, adding packages, interfaces, stores or handlers, restructuring or refactoring a Go repo, or reviewing its structure, including requests for "MVC", "clean architecture" or "how should I structure this". Small tools get a minimal flat layout. Not for Go syntax questions or fixes contained inside one function.
---

# Go architecture

The goal is Go code a senior reviewer would call clean and that keeps
scaling: easy to read in one sitting, easy to change in one place, and
able to grow to dozens of domains and several teams without a rewrite.

Two failure modes ruin Go codebases; this skill exists to avoid both:

1. **Too little structure for the size:** one giant package, globals,
   SQL in handlers, import cycles patched with hacks.
2. **Too much structure for the size:** clean-architecture ceremony on a
   small program: ports, adapters, mocks, five hops per request,
   interfaces with one implementation.

The fix for both: **match structure to the codebase's real stage, and
move up only when a concrete trigger fires.** Go refactors cheaply
because the compiler finds every caller, so deferring structure costs
little, while speculative structure is paid for on every change.

## Workflow

1. **Read before writing.** Look at `go.mod`, the directory tree, one
   existing feature end to end, and any `CLAUDE.md`, `ARCHITECTURE.md`
   or `docs/adr/`. Existing consistent conventions win over this skill;
   two styles in one repo is worse than either. If the conventions are
   actively harmful (cycles, globals, SQL in handlers), say so and
   propose a stepwise migration instead of silently adding a third style.
2. **Pick the stage** (below).
3. **Place the code.** Use the stage's layout from
   `references/layout.md`. An endpoint, job or query lives in the domain
   that owns the data it returns or changes, whatever its URL looks like:
   `GET /customers/{id}/invoices` belongs to `invoice`, not `customer`.
4. **Check the dependency rules and red flags** below.
5. **Write tests** per `references/testing.md`.
6. **Verify before saying it's done:** `gofmt`, `go vet ./...`,
   `golangci-lint run` (if the repo has a config) and `go test -race` on
   the touched packages plus the architecture tests. Fix findings in the
   code; never silence a linter or skip a test to get green. If a check
   can't run here (no Docker for database tests, no network, no
   golangci-lint), name each check that didn't run and why, call the
   work unverified, and give the command to run; never report it as
   done or passing. Never add skip-if-unavailable logic to make a
  missing dependency look green (`references/testing.md`).
7. **Say what structure you added and which trigger justified it,** in
   one or two sentences. In a Stage 3 repo, offer an ADR in `docs/adr/`.
8. **Turn corrections into rules.** Fix the code first. Then, when the
   correction is stated as general ("always", "never", "I told you
   last time"), encode it before the task closes so it isn't repeated
   next session. Use a check in `arch_test.go` or a depguard rule only
   if it would raise no false positives on code the user would accept;
   otherwise add one line to the repo's `CLAUDE.md`. Say which you
   changed. A one-off preference about this change stays in the chat.

## Stages

| Stage | Shape | Typical fit |
|---|---|---|
| 1. Flat | One `main` package, or `main` plus one package. Concrete types, no interfaces. | Tools, CLIs, prototypes, under ~2-3k lines with one concern |
| 2. Feature packages | `cmd/<binary>/main.go` wires everything; one package per domain under `internal/`; infrastructure under `internal/platform/`. | One deployable, several domains, a handful of contributors |
| 3. Enforced boundaries | Stage 2 plus: each domain's internals hidden from other domains, import rules checked in CI, owners per domain, events for cross-domain side effects, several binaries. | Many domains, several teams, long-lived |

**Choosing the starting stage for new code:**

- A tool or CLI starts at Stage 1, even if polished.
- A service the user expects to grow (a "real system", "will get big",
  "several teams later") starts at **Stage 2 with Stage-3-ready habits**:
  each domain owns its tables, other domains use only its exported API,
  domains refer to each other by ID, and the example's `.golangci.yml`
  and `arch_test.go` are copied in on day one (with the module path
  changed). Planned teams are not a trigger for Stage 3 machinery; real
  teams are.

**Triggers for adding structure (and only these):**

| Trigger | Add |
|---|---|
| A second binary (worker, migrator, CLI next to the API) | `cmd/<name>/`; Stage 2 if not there yet |
| A Stage 1 program passes ~2-3k lines or has two unrelated concerns | Stage 2 |
| A second domain whose rules don't belong with the first | A new package under `internal/` |
| Two packages need each other | Break the cycle (`references/boundaries.md`), never a `common` package |
| A second real implementation; a dependency that can't run in tests (third-party API, payments, email, clock); or another domain that is owned by a different team or expected to move out of process | A small interface declared by the consumer |
| A package past ~15-20 files or with unrelated reasons to change | Split by sub-domain, never by layer |
| A second team, or other domains reaching into a domain's internals | Stage 3 enforcement |

"We might need it later" is not a trigger.

## Dependency rules

- **Business logic is pure of transport and storage.** Within a domain
  package, adapters live in files named `http*.go` and `store*.go`;
  every other file holds types and rules and imports no `net/http`, no
  `database/sql`, no driver, no vendor SDK and no `platform/postgres`.
  Services make writes atomic through `store.InTx`, never touch the
  store's connection, and never call an external system inside a
  transaction (commit, then call with an idempotency key, then record
  the result). The
  naming convention is what lets one file-scoped lint rule cover every
  current and future domain (see `references/boundaries.md`), so keep to
  it. Clients for external systems (payments, email, other companies'
  APIs) always live in `internal/platform/<system>`, never in a domain.
  Domains sit side by side under `internal/`, never nested in each other.
- **`platform/` imports no domain.** If shared infrastructure needs
  domain knowledge, that code belongs in the domain.
- **Domains refer to other domains by ID**, not by embedding each
  other's structs. `order.Order` has a `CustomerID string`, not a
  `customer.Customer`. Embedding is how package-by-layer codebases end
  up with cycles.
- **`main` is the only place that knows everything.** It reads config,
  opens connections, builds every component with plain constructors and
  owns the lifecycle. No `init()` side effects, no package-level mutable
  state, no DI containers or service locators.
- **Accept interfaces, return structs.** Constructors return concrete
  types. Interfaces are declared by the consumer, sized to what it calls,
  and justified by a trigger above.
- **Cross-domain default: import directly.** Domain A calls
  `*b.Service` from B's exported API. Switch to a consumer-declared
  interface only when the interface trigger above fires; then make its
  contract self-contained (`references/boundaries.md`). One default,
  so code doesn't flip between styles from one session to the next.
- **Each domain owns its tables.** Others go through its exported API,
  never its SQL.
- **`context.Context` is the first parameter** of anything that does
  I/O or blocks; never stored in a struct.

## MVC, translated to Go

When someone asks for "MVC" they want HTTP, logic and data separated.
Give them that separation inside each feature package:

| MVC term | Go equivalent | Lives in |
|---|---|---|
| Controller | Handler: decode, call service, map errors, encode | `internal/order/http.go` |
| Model (logic) | Domain types and `Service` with the business rules | `internal/order/order.go`, `service.go` |
| Model (data) | `Store`: SQL or generated queries for the domain's own tables | `internal/order/store.go` |
| View | Response structs, or `html/template` files for server-rendered pages | `http.go`, `web/templates/` |

Never create top-level `models/`, `controllers/`, `views/`, `services/`
or `repositories/` packages: every feature change then touches every
package, related models import each other into cycles, and names
stutter (`models.User`).

## Default stack

Use what the repo already uses. For new code: the current stable Go
release in `go.mod` (Go supports only the two newest releases, so never
start below the older of those), stdlib `net/http`
`ServeMux` (method and path patterns), `log/slog`, Postgres through
`database/sql` with the pgx driver (or `pgxpool` directly), `sqlc` once
there are more than a handful of queries, one migration tool (goose or
golang-migrate), golangci-lint v2 in CI, OpenTelemetry at Stage 3. Add a
router or framework only when the stdlib is clearly missing something
the project needs.

## Red flags to refuse or fix

- Packages named `utils`, `helpers`, `common`, `shared`, `base`,
  `types`, `models`. Name packages for what they provide.
- Interfaces with one implementation and no trigger; interfaces declared
  next to their implementation "for mocking".
- Stutter: `order.OrderService` instead of `order.Service`.
- SQL, `*http.Request` or `*sql.Tx` in business logic or domain
  signatures.
- Logging an error and also returning it; handle it once.
- Getters and setters on plain data.
- `pkg/` in an application repo (it means "importable by outsiders").
- Functional options on internal application types; a config struct is
  simpler. They're fine for reusable clients and libraries with many
  optional knobs.
- Goroutines with no owner that stops and waits for them.
- Generic `Repository[T]` layers over a real database.
- A connection, `DBTX` or `*sql.Tx` reachable from a service, including
  by embedding `*sql.DB` in a wrapper type.

## Reference files

Read only what the task needs:

- `references/layout.md`: directory trees for each stage (including CLI
  shape), file roles inside a feature package, naming, top-level
  directories. Read for new projects, new packages, or restructuring.
- `references/boundaries.md`: interfaces, wiring, cross-domain calls,
  breaking cycles, cross-domain transactions, events, Stage 3 `internal/`
  rules and a working depguard config. Read when packages interact or a
  boundary is added.
- `references/persistence.md`: the Postgres store pattern, the `DBTX`
  and transaction helper, error translation, sqlc and migrations. Read
  whenever code touches a database.
- `references/errors-and-context.md`: error design, the shared error
  envelope, HTTP API conventions (pagination, nested routes, status
  codes), logging with request IDs, middleware, context, shutdown,
  concurrency, observability and auth. Read when writing handlers,
  endpoints, services or `main`.
- `references/testing.md`: what to test at which level, real
  dependencies over mocks, test style, what to do when a check can't
  run. Read whenever writing tests.
- `references/review-checklist.md`: checklist and stepwise refactor
  recipes. Read for review, "clean this up", or restructuring requests.

## Golden example

`examples/shop/` is a compiling Stage 2 service with two domains. It
shows the defaults, not the exceptions:

- `order` calls `catalog` directly through its exported `*Service`
  (batched `Prices`), the default for cross-domain calls.
- The only interface is `order.PaymentGateway`, for the one dependency
  that can't run in tests. Its adapter lives in `platform/payments`; a
  hand-written fake stands in for it in tests.
- Money-moving idempotency. `POST /orders` honours a client
  `Idempotency-Key`: a retry with the same body replays the original
  outcome instead of creating a second order, a different body under the
  same key is rejected, and an unpaid original answers 409 with the
  `/pay` link. (No authentication here, so keys are global; with
  authentication, scope them per caller.) This machinery is for
  endpoints that move money; copy it only there. Ordinary create and
  read endpoints get none of it (no request key, hash, attempt counter
  or retry endpoint) until a client actually retries them, and follow
  `catalog`'s plain shape instead. The order is saved, then charged
  outside any transaction. Provider keys name one attempt of one
  operation (`order-<id>-charge-<n>`): reused after an unknown outcome,
  replaced after a definite decline, never shared with a refund. A
  failed charge leaves a pending order that `POST /orders/{id}/pay`
  retries; declines (402) are told apart from outages (502) without
  importing provider types.
- Wiring in `cmd/api/main.go`; request-ID, recovery and logging
  middleware; one error envelope in `platform/web`; graceful shutdown.
- `.golangci.yml` (file-scoped boundary rules, strict allow-list for
  logic files) and `arch_test.go` (package names, stutter, `init()`,
  globals, SQL outside stores, cross-domain struct fields, nested
  domains), both passing with zero issues.

Its stores are in-memory to keep it dependency-free; real stores follow
`references/persistence.md`, including `store.InTx`. Copy its shape,
not its domain.
