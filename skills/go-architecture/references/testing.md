# Testing

## Contents
- What to test where
- Real dependencies over mocks
- Test style
- Integration tests and CI

## What to test where

| Level | Tests | Uses |
|---|---|---|
| Domain rules (`<domain>.go`) | Invariants and pure functions, table-driven | Nothing external |
| Service | Use cases end to end within the domain | Real store, real sibling domains; fakes only for what you can't run |
| Handler | Routing, status codes, decoding, response shape, error envelope | `httptest`, a real service built for the test |
| Store | SQL correctness, constraints, not-found and duplicate mapping | Real Postgres (testcontainers or a CI service container) |
| Wiring | The graph `main` builds serves `/healthz` and a happy path; `run` shuts down on cancel | `httptest` on the handler factory; `run` with an already-cancelled context |

Most value sits in service and handler tests. Don't test the same rule
at every level.

To make wiring testable, split `main.go` into `run` (config, listener,
lifecycle) and a `newHandler(deps...) http.Handler` that builds the
graph. Test the handler with `httptest.NewServer`. Never `time.Sleep`
to wait for start-up; with a listener-based `Serve`, nothing needs
waiting for.

## Real dependencies over mocks

- Test against the real implementation whenever it can run locally:
  your store against a real database, the real `catalog.Service` when
  testing `order`.
- Fakes are for boundaries you don't own or can't run: payment
  providers, email, external APIs, injected failures. Write them by
  hand as small structs in the test file; they're tiny because the
  consumer interface is small.
- Avoid generated mocks that assert call sequences: they test the
  implementation, break on every refactor, and pass when the real
  dependency would fail.
- Control time and IDs by injecting `func() time.Time` and
  `func() string`, not by mocking.

## Test style

- Table-driven with named cases and `t.Run` when cases share shape; a
  single straightforward test needs no table.
- Black-box (`package order_test`) for the public API; white-box only
  for tricky internals.
- Compare with `==`, `maps.Equal`, `slices.Equal`, or
  `github.com/google/go-cmp/cmp` for structs. Avoid assertion libraries
  that hide the comparison.
- Messages state got and want: `t.Errorf("TotalCents() = %d, want %d", got, want)`.
- Helpers take `t` first and call `t.Helper()`.
- `t.Cleanup`, `t.TempDir`, `t.Setenv`, `t.Context()` instead of
  hand-rolled teardown.
- `t.Parallel()` in tests and subtests that share no state, which with
  per-test fixtures is almost all of them.

## Integration tests and CI

- Store and end-to-end tests that need Postgres run against a real
  instance. Start **one** container per test binary in `TestMain` and
  give each test its own schema or a transaction rolled back in
  `t.Cleanup`; that keeps a suite of hundreds of service tests in
  seconds, not minutes. Skip only the slowest end-to-end tests under
  `testing.Short()`, not every test that touches the database.
- CI order: `go build ./...`, `go vet ./...`, `golangci-lint run`,
  `go test -race ./...` with the database available.
- Coverage is a signal for untested domain rules, not a target.
- `arch_test.go` at the module root (copied from the example) runs
  with every `go test ./...`. It reads the module path from `go.mod`;
  if the root directory already holds a Go package, change the
  file's package clause to match. When a rule there is wrong for this
  codebase, change the rule in the same PR and say why; don't skip it.
- When Postgres isn't available (no Docker for testcontainers), first
  look for `TEST_DATABASE_URL` or a local instance. If there is none,
  run what you can (build, vet, `arch_test.go`, and the packages that
  don't need a database), then list each check that didn't run and the
  command to run it. Database tests must fail, not skip, when Postgres
  is missing, so CI can't pass by accident: don't add skip-if-no-Docker
  logic, don't swap the real store for a mock to get green, and don't
  call the work verified.
- Generated or scaffolded code must pass the repo's `golangci-lint run`
  with zero issues; fix the code rather than disabling a linter, and if
  a rule is genuinely wrong for the codebase, exclude it in config with
  a comment saying why. The golden example passes its own config.
