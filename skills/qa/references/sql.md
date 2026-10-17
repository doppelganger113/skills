# SQL databases, migrations and queries

PostgreSQL detail is given where behaviour differs between databases; for MySQL, SQLite and others, check the equivalent rule for that database and version before reporting.

Contents
- Static checks
- Migrations and locking (PostgreSQL)
- Migrations and rollouts (any database)
- MySQL and SQLite notes
- Transactions and concurrency
- Query correctness
- Capacity

## Static checks

Never run migrations, and never connect to a database, including for `EXPLAIN`.

| Command | Class | Notes |
|---|---|---|
| `squawk <migration files>` | S if installed | PostgreSQL migration linter; reads SQL files only. |
| `sqlfluff lint <paths>` | S only with the default templater and no `library_path`; otherwise E | The Jinja and Python templaters can import repository code. |
| migration tool commands (`migrate`, `alembic`, `flyway`, `atlas`, ORM `makemigrations`) | Never | They connect to databases or write files. |

## Migrations and locking (PostgreSQL)

Most DDL takes an `ACCESS EXCLUSIVE` lock. The lock waits behind any running query on the table, and every later query queues behind the waiting lock, so even an instant change can stall a busy table. Look for `lock_timeout` (and `statement_timeout`) set for the migration.

- `ADD COLUMN` with a volatile default (`gen_random_uuid()`, `clock_timestamp()`) rewrites the table. A constant or stable default (`now()`) doesn't, since PostgreSQL 11.
- `ADD COLUMN ... NOT NULL` without a default fails on a table that has rows.
- `ALTER COLUMN ... SET NOT NULL` scans the whole table under `ACCESS EXCLUSIVE`, unless a validated `CHECK (col IS NOT NULL)` constraint already exists (PostgreSQL 12+).
- `ALTER COLUMN ... TYPE` usually rewrites the table and its indexes.
- `CREATE INDEX` without `CONCURRENTLY` blocks writes for its duration. `CREATE INDEX CONCURRENTLY` can't run inside a transaction block, and many migration tools wrap each migration in one; if it fails it leaves an `INVALID` index behind.
- `ADD FOREIGN KEY` or `ADD CONSTRAINT ... CHECK` validates every row while holding locks; `NOT VALID` followed by `VALIDATE CONSTRAINT` in a later step avoids that.
- A large `UPDATE` or `DELETE` backfill in one transaction: long locks, table bloat, WAL volume and replication lag. Batch it.

## Migrations and rollouts (any database)

- Old and new application versions run at the same time during a rolling deploy. Renaming or dropping a column or table, or adding a required column, breaks the old version. The safe order is expand (add, write both), migrate, then contract in a later release.
- Migration and code deployed in the wrong order, or a migration that assumes code that hasn't shipped.
- Down migrations that lose data, or that can't run after new data has been written.
- `ALTER TYPE ... ADD VALUE` for enums: older versions refuse it inside a transaction, and a new value can't be used in the same transaction that adds it. Old readers may reject new values.
- Seed or reference data changed by a migration that differs between environments.

## MySQL and SQLite notes

- MySQL: DDL that can't use `ALGORITHM=INSTANT` or `INPLACE` copies the table; any DDL waits for a metadata lock behind long transactions and blocks queries queued after it.
- SQLite: one writer at a time. Concurrent writers without a busy timeout get `database is locked`; long transactions block every other writer.

## Transactions and concurrency

- Read-then-write in application code (check a balance, then update it; check a name is free, then insert it) under the default isolation (Read Committed in PostgreSQL, Repeatable Read in MySQL InnoDB): two transactions both pass the check. Fix with `SELECT ... FOR UPDATE`, a unique or check constraint, an atomic `UPDATE ... WHERE`, or an upsert.
- An invariant like "one active X per Y" enforced only in code: a partial unique index enforces it.
- `SERIALIZABLE` or `REPEATABLE READ` without retrying serialization failures (SQLSTATE `40001`), and deadlocks (`40P01`) from locking rows in inconsistent order.
- A transaction held open across a network call or user think-time: locks held, connections pinned, sessions stuck idle in transaction.
- `ON CONFLICT DO NOTHING` that silently swallows the case the caller needed to know about.

## Query correctness

- `NOT IN (subquery)` returns no rows if the subquery yields a `NULL`; `NULL = NULL` is not true; `COUNT(col)` skips `NULL`s.
- `LIMIT` and `OFFSET` pagination without a unique `ORDER BY`: rows repeat or go missing between pages, and deep offsets get slow.
- `timestamp` without time zone storing values from servers in different zones; comparisons around DST.
- Case-insensitive uniqueness (emails, usernames) enforced on the raw column.
- `ON DELETE CASCADE` deleting more than the caller expects; soft deletes that unique constraints don't account for.
- SQL built by string concatenation with any input.

## Capacity

- `integer` (`int4`) primary keys or sequences on fast-growing tables overflow at about 2.1 billion.
- Missing indexes on foreign keys and on columns used in frequent filters or joins; `LIKE '%term'` can't use a normal index.
- Connection counts: pool size per instance times replicas and workers against the database's connection limit.
- Unbounded result sets loaded into memory.
