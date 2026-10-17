# Failure modes: the stack-independent checklist

Contents
- How to use this list
- 1. Inputs and boundaries
- 2. State and invariants
- 3. Concurrency
- 4. Partial failure, retries and timeouts
- 5. Data changes and compatibility
- 6. Security and trust boundaries
- 7. Resource limits
- 8. Dependencies, configuration and environment
- 9. Observability
- 10. Rollout and rollback
- 11. Plans: what plan mode adds
- 12. Conformance: what conformance mode adds

## How to use this list

Walk it at each boundary in scope: every place data enters, crosses a process or network, gets stored, or triggers work (a request handler, a database write, a queue message, a file, a scheduled job, a call to another service). Skip categories that don't apply. Each question is a prompt to look for a concrete trigger, not a finding in itself: a finding needs the inputs, state or timing that produce a wrong outcome, and the line where it happens.

## 1. Inputs and boundaries

- Empty, missing, null, zero, negative, one element, exactly at a limit, one past it.
- Very large: a payload, a list, a string, a number near its type's limit, a file. Who caps it, and where?
- Duplicates, unexpected order, the same request twice.
- Malformed or adversarial: wrong type, invalid encoding, Unicode normalisation and case, leading or trailing whitespace, control characters.
- Time: time zones, DST transitions, month ends, leap days, clock skew between machines, timestamps without a zone, durations crossing midnight.
- Money and precision: floating point for currency, rounding direction, currency units.
- Identifiers: assumptions that IDs are numeric, sequential, unique across tenants, or fit in the client's number type.

## 2. State and invariants

- What must always be true (a balance never negative, one active subscription per user, an order total equals its lines)? Is it enforced where it can't be bypassed, such as a database constraint, or only by application code that a second path can skip?
- Multi-step updates: if the process dies between step 2 and step 3, what state is left, and does anything repair it?
- Caches and derived data: when do they go stale, who invalidates them, and what reads a stale value and acts on it?
- State machines: transitions from every state, including terminal ones, and events arriving in an unexpected state.

## 3. Concurrency

- Two requests, workers or job runs doing the same thing at once: read-modify-write, check-then-act, double submit, two scheduled runs overlapping.
- Shared mutable state across threads, goroutines, async tasks or processes.
- Ordering: does the code assume messages, events or callbacks arrive in order, or exactly once?
- Lock order and deadlocks; locks held across network calls or user code.
- Cancellation: what happens to work in flight when the caller gives up?

## 4. Partial failure, retries and timeouts

- Every external call can fail, hang, or succeed while its response is lost. What does the code do in each case?
- Retries: is the retried operation idempotent? Is there an idempotency key, a unique constraint or a dedupe check? Backoff, jitter and a cap, or a retry storm when the dependency is down?
- Timeouts: does every network call have one, and do they fit inside the caller's own deadline?
- Dual writes (database plus queue, database plus external API) without an outbox or reconciliation: what happens when one side succeeds and the other fails?
- Messages: at-least-once delivery, poison messages that fail forever, dead-letter handling.
- Crash and restart: what happens to in-memory work, half-written files and held leases?

## 5. Data changes and compatibility

- During a rolling deploy, old and new code run at the same time: can both read and write the data each other produces?
- Schema and format changes: renamed or removed fields, new required fields, new enum values that old readers reject, changed serialisation.
- Backfills: size, locking, duration, what runs against the data while they run.
- Irreversible steps: dropping data or columns, overwriting in place. Is there a way back?

## 6. Security and trust boundaries

- Authentication and authorisation on every path, including the new one, batch jobs and internal endpoints; object-level checks (can user A read user B's record by changing an ID?).
- Injection wherever input reaches an interpreter: SQL, shell, templates, file paths, URLs fetched by the server (SSRF), regular expressions, deserialisation.
- Secrets in logs, errors, URLs, client bundles, images or committed files.
- Multi-tenancy: queries and caches scoped by tenant everywhere.
- Rate limits and abuse on anything expensive or that sends messages.
- Defaults: what a missing config value or an unset flag grants.

## 7. Resource limits

- Unbounded growth: queues, caches, maps, buffers, goroutines, threads, tasks, open files, connections, temp files, logs on disk.
- Per-request cost: memory proportional to input size, N+1 queries, full scans, loading a whole result set.
- Pools: connection or worker pool exhaustion under load or when a dependency slows down.
- Dependency quotas and rate limits, and what happens when they are hit.

## 8. Dependencies, configuration and environment

- Missing or invalid configuration: fail fast at startup, or fail later on the first request?
- Feature flags: both states work, and switching mid-flight is safe.
- Differences between environments: a dependency that exists locally but not in production, file system layout, permissions, time zone of the server.
- Third-party behaviour the code relies on but nothing guarantees: ordering, pagination, error codes, eventual consistency.

## 9. Observability

- If this fails in production, would anyone know? Is the failure path logged, counted or alerted, or swallowed?
- Can on-call tell what happened from the logs and metrics alone: IDs, the failing dependency, the input that triggered it?
- Health checks that reflect real readiness, not just that the process is up.

## 10. Rollout and rollback

- Deploy order between services, migrations and clients.
- Can it be rolled back after the data has changed? What breaks if it is?
- A kill switch or flag for risky behaviour, and a canary or staged rollout where the blast radius is large.

## 11. Plans: what plan mode adds

- Unstated assumptions the plan depends on (load, data size, ownership, ordering, "this is reversible"), and which of them the repository contradicts.
- Missing decisions: what happens when a step fails halfway, who owns a component, what the error behaviour is, what "done" means.
- Steps that have no failure handling or no way back, and the order between steps that depend on each other.
- Success criteria and tests: are they measurable, and do the tests cover failure paths and the edge cases in sections 1 to 10, not only the happy path?
- Scope gaps: components, clients, jobs or environments the plan touches without saying so.

## 12. Conformance: what conformance mode adds

- Each requirement in the plan: implemented, missing, or implemented differently.
- Things the code does that the plan rules out, or that the plan never mentions and that change behaviour for users or other systems.
- Failure handling the plan specifies (timeouts, retries, fallbacks, limits): present, with the values the plan names?
