# Persistence

## Contents
- Rules
- Platform: DBTX and transactions
- A domain store
- Transactions: the service decides, the store executes
- Across domains
- sqlc
- Migrations

## Rules

- Each domain has its own `Store` over its own tables. No other domain
  reads those tables; it calls the domain's service instead.
- SQL lives only in `store*.go` (or generated code the store wraps).
  Only store files import `database/sql`, the driver, or
  `platform/postgres`; the lint rule enforces this.
- The store translates driver errors into domain errors
  (`sql.ErrNoRows` becomes `ErrNotFound`, a unique violation becomes
  `ErrDuplicate`). Nothing above the store sees driver errors.
- The service decides what must be atomic by calling `store.InTx` and
  never touches a connection, `DBTX` or `*sql.Tx`. The compiler can't
  enforce this (service and store share a package), so `arch_test.go`
  checks it: `TestOnlyStoresRunSQL` flags reaching into a store's
  unexported fields (`s.store.q`, which also covers pgx and sqlc
  methods) and `database/sql`/pgx query calls outside `store*.go`. It is
  a strong tripwire, not a proof; code review covers the rest.
- **Never call an external system inside `InTx`.** A transaction held
  open across a network call ties up a connection, and a rollback can't
  undo a charge or an email. Commit first, then call the provider with
  an idempotency key, then record the result in a second write. The
  example's save-pending, charge, mark-paid flow is the pattern.
- Query by ID or by explicit filter structs; don't build a generic
  query layer.

## Platform: DBTX and transactions

```go
// Package postgres owns the connection pool and transactions. It knows
// nothing about any domain, and only store files may import it.
package postgres

// DBTX is satisfied by both *sql.DB and *sql.Tx, so a store runs the
// same queries inside or outside a transaction.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DB wraps the pool. The pool is unexported on purpose: queries go
// through Q, and only store files can reach Q.
type DB struct {
	pool *sql.DB
}

// New wraps an open pool.
func New(pool *sql.DB) *DB { return &DB{pool: pool} }

// Q returns the pool as a DBTX for queries outside a transaction.
func (db *DB) Q() DBTX { return db.pool }

// InTx begins a transaction, runs fn, and commits if fn returns nil.
// The deferred Rollback undoes everything if fn fails or panics; after
// a successful Commit it is a no-op.
func (db *DB) InTx(ctx context.Context, fn func(q DBTX) error) error {
	tx, err := db.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
```

`DBTX` is an interface with two implementations (`*sql.DB`, `*sql.Tx`),
which is exactly the trigger for one. Don't embed `*sql.DB` in `DB`:
embedding would hand every importer `ExecContext` and reopen the hole
the lint rule is meant to close. With `pgxpool`, the same shape uses
pgx's `Exec`/`Query`/`QueryRow` signatures, as sqlc generates.

## A domain store

```go
// Store persists orders in Postgres. It owns the orders and
// order_lines tables; no other domain reads them.
type Store struct {
	db *postgres.DB
	q  postgres.DBTX
}

// NewStore returns a store that runs queries on the pool.
func NewStore(db *postgres.DB) *Store {
	return &Store{db: db, q: db.Q()}
}

// InTx runs fn with a copy of the store bound to one transaction.
func (s *Store) InTx(ctx context.Context, fn func(tx *Store) error) error {
	return s.db.InTx(ctx, func(q postgres.DBTX) error {
		return fn(&Store{db: s.db, q: q})
	})
}

// Get returns the order with the given ID, or ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (Order, error) {
	o := Order{ID: id}
	err := s.q.QueryRowContext(ctx,
		`SELECT created_at FROM orders WHERE id = $1`, id).Scan(&o.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound // driver errors stop here
	}
	if err != nil {
		return Order{}, fmt.Errorf("select order: %w", err)
	}

	rows, err := s.q.QueryContext(ctx,
		`SELECT sku, quantity, unit_price_cents FROM order_lines
		 WHERE order_id = $1 ORDER BY sku`, id)
	if err != nil {
		return Order{}, fmt.Errorf("select order lines: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var l Line
		if err := rows.Scan(&l.SKU, &l.Quantity, &l.UnitPriceCents); err != nil {
			return Order{}, fmt.Errorf("scan order line: %w", err)
		}
		o.Lines = append(o.Lines, l)
	}
	if err := rows.Err(); err != nil {
		return Order{}, fmt.Errorf("iterate order lines: %w", err)
	}
	return o, nil
}
```

## Transactions: the service decides, the store executes

```go
func (s *Service) Place(ctx context.Context, lines []LineRequest) (Order, error) {
	// ... validate, price ...
	err := s.store.InTx(ctx, func(tx *Store) error {
		return tx.Insert(ctx, o) // order row + line rows, atomically
	})
	if err != nil {
		return Order{}, fmt.Errorf("save order: %w", err)
	}
	return o, nil
}
```

`main` builds `db := postgres.New(pool)`, then
`order.NewService(order.NewStore(db), ...)`. The service imports
neither `database/sql` nor `platform/postgres`, and nothing inside the
`InTx` callback calls out of the process.

## Across domains

Prefer one domain per transaction plus an outbox (`boundaries.md`).

If a use case truly must change two domains' tables atomically (Stage
2, one database), the coordinating domain's store exposes one explicit
method that binds both stores to the same transaction, and the other
domain exports a constructor for that purpose:

```go
// order/store.go
// InTxWithCatalog exists because placing an order must reserve stock
// atomically. Keep this the only such method in the domain.
func (s *Store) InTxWithCatalog(ctx context.Context,
	fn func(tx *Store, cat *catalog.Store) error) error {
	return s.db.InTx(ctx, func(q postgres.DBTX) error {
		return fn(&Store{db: s.db, q: q}, catalog.StoreOn(q))
	})
}

// catalog/store.go
// StoreOn binds a catalog store to a transaction another domain opened.
func StoreOn(q postgres.DBTX) *Store { return &Store{q: q} }
```

The coupling is then visible in one named place. At Stage 3, needing
this is a sign the boundary is wrong.

## sqlc

- Adopt once a domain has more than a handful of queries.
- Generate into the owning domain: `internal/order/internal/db/` with
  `emit_interface: false` and the domain's own `queries.sql`.
- `store.go` wraps the generated `*db.Queries` and maps generated row
  types to domain types; generated types never leave `store.go`.
- One `make gen` (or `go generate ./...`) target regenerates
  everything; generated code is committed.

## Migrations

- One `migrations/` directory per database, numbered files, run by a
  dedicated `cmd/migrate` binary or a startup step that is off by
  default in production.
- Migrations are forward-only in production; write the down migration
  for local development only if the tool needs it.
- A domain's migrations touch only that domain's tables. Prefix table
  names or use one Postgres schema per domain at Stage 3.
