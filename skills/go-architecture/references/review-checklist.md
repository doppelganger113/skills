# Review and refactoring checklist

For "review this", "clean this up" or "how should I restructure this"
on an existing Go codebase.

## How to review

1. Map the import graph, including broken parts:
   `go list -e -f '{{.ImportPath}}: {{join .Imports " "}}' ./...`
   (`-e` keeps going past the cycles you're trying to find; filter to
   the module's own packages). `go build ./...` names each cycle.
2. Identify the stage the code is at and the stage its size needs.
3. Read one feature end to end, route to SQL.
4. Report findings ordered by cost of leaving them: correctness and
   cycles, then boundaries, then naming and style.
5. Propose refactors as small, separately mergeable steps, each
   compiling and passing tests. Never a big-bang restructure.

## Checklist

**Structure**
- [ ] Packages by feature, not by layer.
- [ ] No `utils`/`common`/`models`/`types` packages.
- [ ] Structure matches the stage; nothing without a trigger.
- [ ] `main` wires everything through plain constructors; no `init()`
  side effects, globals or DI containers.
- [ ] Every endpoint lives in the domain that owns its data.

**Boundaries**
- [ ] Business logic files import no `net/http` or `database/sql`.
- [ ] `platform/` imports no domain.
- [ ] Domains reference each other by ID, not embedded structs.
- [ ] Interfaces small, consumer-defined, self-contained, justified.
- [ ] No domain reads another domain's tables.
- [ ] Import graph acyclic without workaround packages.
- [ ] Transactions opened with `store.InTx` from services, never in handlers.
- [ ] Clients for external systems live in `internal/platform/<system>`.

**Errors and context**
- [ ] Errors wrapped with context, handled once.
- [ ] Driver errors translated at the store.
- [ ] No typed nil pointer returned as an `error`.
- [ ] One error envelope; each handler maps only its domain's errors.
- [ ] `ctx` first on I/O functions, never stored.

**Lifecycle and concurrency**
- [ ] Graceful shutdown; server timeouts set.
- [ ] Every goroutine has an owner and a stop path.
- [ ] Fan-out is bounded; no lock held across I/O or a callback.
- [ ] Tests pass with `-race`.

**Readability**
- [ ] No stutter; names read well with the package prefix.
- [ ] Exported identifiers have doc comments starting with the name.
- [ ] Early returns; happy path unindented.
- [ ] Files under ~500 lines; functions do one thing.

**Tests**
- [ ] Service and handler tests use real implementations where possible.
- [ ] Fakes only at boundaries you don't own or to inject failures.
- [ ] Store tests hit a real database.

## Refactor recipes

**From `models/`, `controllers/`, `services/` to feature packages**

1. Map which model structs embed or reference each other (`User.Orders`,
   `Order.User`). These references are what cause the cycles.
2. Order domains by dependency and migrate **leaf domains first** (the
   ones nothing else references), one PR each.
3. For each domain: create `internal/<domain>/`, move its model,
   service, store and controller together, rename (`models.User` to
   `user.User`, `UserService` to `user.Service`), fix imports.
4. Replace references to another domain's structs with IDs; where a
   response needs the other domain's data, fetch it through that
   domain's service and compose it in the handler or service.
5. Delete the empty layer packages; add the depguard rules so the old
   shape can't creep back.

**Other smells**

| Smell | Steps |
|---|---|
| SQL in handlers | Extract a `Store` method per query, then a `Service` method per use case, then thin the handler |
| Interface per struct for mocking | Delete interfaces with one implementation; rewrite those tests against the real implementation; keep interfaces only where a trigger applies |
| Global DB or logger | Add constructor parameters, pass from `main`, delete the global |
| Per-item cross-domain calls in a loop | Add a batched method to the provider; change the consumer interface to it |
| Cycle between two domains | Follow "Breaking import cycles" in `boundaries.md` |
