# Go

Contents
- Static checks
- Concurrency and goroutines
- Context, timeouts and shutdown
- Errors and panics
- Resources
- Data and types
- Version-dependent behaviour

## Static checks

Run every Go command with `GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-mod=readonly`, so a `toolchain` line in `go.mod` can't download and run another Go toolchain and missing modules fail instead of downloading. If modules are missing from the cache, the commands fail; record that.

| Command | Class | Notes |
|---|---|---|
| `go vet ./...` | S | Compiles packages without running them; cgo packages run the C compiler on the repository's C code, with Go's allowlist of compiler flags. |
| `staticcheck ./...` | S | If installed. |
| `golangci-lint run ./...` | S | Only if the repository's `.golangci.*` config loads no plugins (no `custom` linters or module plugins) and the binary is not inside the repository; otherwise E. |
| `go test -race -count=1 ./...` | E | Runs the repository's tests and `TestMain`. |
| `govulncheck ./...` | N | Downloads the vulnerability database. |
| `go generate`, `go mod tidy`, `go get`, `gofmt -w`, `go fix` without `-diff` | Never | They write files or download. |

Findings that `go vet` reports by default (copied locks, lost context cancel, printf mistakes, unreachable code) belong to the tool; don't repeat them.

## Concurrency and goroutines

- A goroutine blocked forever on a channel send or receive after its partner returned early (an error path, a timeout, a cancelled context): a leak per request.
- `sync.WaitGroup.Add` called inside the goroutine instead of before `go`, racing with `Wait`.
- Concurrent writes to a map: a fatal error that `recover` can't catch.
- Shared struct fields or slices written from several goroutines without a lock or channel.
- A panic in a goroutine started from an HTTP handler crashes the whole process; `net/http` recovers only the handler's own goroutine.
- Unbounded fan-out (`go` per item of user-controlled length) with no semaphore or `errgroup.SetLimit`.
- `select` with a `default` branch inside a loop: a busy loop burning CPU.
- `errgroup.WithContext`: the derived context is cancelled when the first goroutine fails; code that keeps using it after `Wait` sees a cancelled context.

## Context, timeouts and shutdown

- `http.Client{}` or `http.DefaultClient` with no `Timeout`, and requests built without a context: a hung dependency hangs the caller forever.
- `http.Server` with no `ReadHeaderTimeout` (or `ReadTimeout`): slow clients hold connections open.
- `context.Background()` or `context.TODO()` in a request path, so cancellation and deadlines don't reach database or HTTP calls.
- Database calls without the `Context` variants (`QueryContext`, `ExecContext`).
- No graceful shutdown: no `signal.NotifyContext`, no `Server.Shutdown`, workers not drained, so a deploy drops in-flight work.
- `log.Fatal` or `os.Exit` after setup skips deferred cleanup such as flushing or closing.

## Errors and panics

- Errors discarded with `_` on operations whose failure matters (writes, `Close` on a written file, `tx.Commit`, `rows.Err`).
- Comparing wrapped errors with `==` instead of `errors.Is` or `errors.As`.
- A function returning a typed nil pointer as an `error` interface: the caller's `err != nil` is true.
- An HTTP handler that writes an error response and then falls through without `return`, writing twice or continuing the operation.

## Resources

- `resp.Body` not closed on every path, or closed without being drained, which prevents connection reuse.
- `rows.Close()` missing on an early return, or `rows.Err()` never checked after the loop.
- `database/sql` with the default unlimited `MaxOpenConns` against a database with a connection limit, or no `SetConnMaxLifetime` behind a proxy or load balancer.
- `defer` inside a loop over files or rows: nothing is released until the function returns.
- `io.ReadAll` or `json.NewDecoder` on an untrusted body without `http.MaxBytesReader` or `io.LimitReader`.
- `time.Ticker` never stopped.

## Data and types

- Writing to a nil map: a panic.
- `append` to a subslice that shares its backing array with the original, overwriting the original's later elements.
- `len(s)` counts bytes, not characters; slicing a string can split a UTF-8 sequence.
- Integer conversions (`int64` to `int32`, `int` to `uint`) wrap silently.
- `encoding/json`: unknown fields are ignored silently, `omitempty` drops legitimate zero values, and `int64` IDs lose precision in JavaScript clients past 2^53.
- `time.Time` compared with `==` instead of `Equal`, or a `time.Time` round-tripped through a database losing its location or monotonic reading.
- A value receiver on a type with a mutex or other state that must not be copied.

## Version-dependent behaviour

Read the `go` line in `go.mod` before reporting these.

- Before Go 1.22, loop variables are shared across iterations, so goroutines or closures that capture them see the last value.
- Before Go 1.23, a `time.After` or `time.NewTimer` that never fires and is never stopped stays in memory until it would have fired; in a loop this grows.
- Before Go 1.25, `GOMAXPROCS` ignores container CPU limits, so a pod with a low CPU limit on a large node is throttled. In any version, a container memory limit without `GOMEMLIMIT` invites OOM kills under load.
