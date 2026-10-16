# go-architecture: why it says what it says

For whoever edits the skill later, most likely its owner. The rules live
in [SKILL.md](../skills/go-architecture/SKILL.md) and its references;
this file records only the reasons, so a future edit doesn't undo a
decision without meaning to. If a reason here stops being true, change
the rule and this file together.

## Contents
- Design decisions
- Rejected approaches
- Maintenance runbook
- Known limits

## Design decisions

**Structure grows in stages, and only when a trigger fires.**
([Stages](../skills/go-architecture/SKILL.md#stages)) Go codebases fail
from too little structure for their size or too much, and a fixed
architecture document causes the second: Claude applies written rules
literally on every task, so a mandatory layer diagram turns a small
service into ports, adapters and mocks. Go refactors cheaply because the
compiler finds every caller, so deferring structure costs little.

**Feature packages instead of MVC packages.**
([MVC, translated to Go](../skills/go-architecture/SKILL.md#mvc-translated-to-go))
Layer packages (`models/`, `controllers/`) make every feature change
touch every package, turn related models into import cycles, and
produce stutter like `models.User`. The MVC split survives inside each
domain as `http.go`, `service.go` and `store.go`.

**Planned teams aren't a trigger; real ones are.** A service expected to
grow starts at Stage 2 with Stage-3 habits, because owning tables and
referring to other domains by ID is cheap from day one and painful to
retrofit, while lint maps, owners and events cost real time before
there's anyone to coordinate.

**Calling another domain directly is the default.**
([Dependency rules](../skills/go-architecture/SKILL.md#dependency-rules))
An earlier version allowed an interface whenever a consumer used "a
narrow slice" of another domain, which is nearly always true. With two
acceptable styles and no tiebreaker, Claude switched between them from
one session to the next. Interfaces now need a named trigger.

**Adapters live in `http*.go` and `store*.go`; logic files have a strict
import allow-list.** The naming lets one file-scoped lint rule cover
every current and future domain with no config edits. The allow-list
replaced a deny-list because a reviewer showed a vendor SDK imported
from a domain file passed the deny-list.

**Clients for external systems always live in `internal/platform/<provider>`.**
One home, so the lint rules need no exceptions. An earlier version
allowed `internal/<domain>/internal/<provider>` too, which the purity
rule then flagged.

**Services never touch the store's connection.** Service and store share
a package, so the compiler can't stop a service calling
`s.store.q.ExecContext`. `TestOnlyStoresRunSQL` in `arch_test.go`
catches it. The `postgres.DB` wrapper doesn't embed `*sql.DB` for the
same reason: embedding hands every importer the query methods.

**No network calls inside a transaction.** A held transaction ties up a
connection, and a rollback can't undo a charge. Hence save, then call
with an idempotency key, then record the result.

**The example's idempotency machinery is for money only.** Request
keys, request hashes, per-attempt provider keys and the `/pay` retry are
there because a payment flow without them double-charges. Claude copies
the example, so SKILL.md says plainly that ordinary endpoints get none
of it. Provider keys are per attempt because a provider replays a
stored decline for a reused key, and a refund must never reuse a
charge's key.

**Real dependencies over mocks.** Generated mocks test the
implementation, break on every refactor, and pass when the real
dependency would fail. Hand-written fakes are kept for what can't run
locally (the payment provider) and for injecting failures.

**Architecture tests use only the standard library.** They then run in
any sandbox and any CI without downloads, which matters because Claude
often works where module downloads are blocked.

**Unrunnable checks are reported, never faked.** Without that rule an
agent in a sandbox with no Docker improvises: skips, mocks, or claims
success. Database tests fail rather than skip when Postgres is missing,
so CI can't pass by accident.

**Corrections become rules, with a false-positive bar.** A correction
that only lives in a chat gets repeated next session. The bar exists
because a brittle check (say, banning `ValidationError` in handlers)
would flag handlers that correctly reject malformed JSON.

**The example's stores are in memory.** That keeps the example
dependency-free and CI-verifiable anywhere. The Postgres pattern in
`references/persistence.md` was compile-checked separately.

**Human docs stay out of the skill folder.** Everything in it is
installed with the skill, and the agent could follow a loose summary
instead of the exact rules. `scripts/check_skills.py` enforces this.

## Rejected approaches

| Approach | Why not |
|---|---|
| Hexagonal or clean architecture as the default | Same mistake as MVC: a layer diagram chosen before the code exists. Ceremony on every small service. |
| Top-level MVC packages | Cycles, stutter, every change touches every package. |
| Generic `Repository[T]` | Hides the SQL that matters and fits no real query well. |
| DI containers and service locators | Hide the object graph; `main` wiring by hand is short and explicit. |
| Generated call-sequence mocks | Test the implementation and break on refactors. |
| `pkg/` in applications | Signals "importable by outsiders" when nothing is. |
| A separate template repo as the source of truth | Claude usually can't reach it, and two copies drift. The example lives in the skill and CI tests it. |
| A human guide inside the skill folder | Ships with the skill and competes with SKILL.md. |

## Maintenance runbook

**The weekly CI run fails but pushes pass.** The scheduled run uses the
newest stable Go and the newest golangci-lint.
- A new `go vet` finding: fix the example.
- A new lint finding: fix the example. If the rule is wrong for this
  code, exclude it in `.golangci.yml` with a comment saying why. Then
  bump the pinned golangci-lint version in
  `.github/workflows/go-architecture.yml`.

**A new Go release comes out.** Go supports the two newest releases.
Move the example's `go.mod` to the older of the two, push, and check
both CI jobs. SKILL.md says "current stable" and needs no change.

**Dependabot opens a PR.** It only bumps the SHA-pinned actions. Merge
it once CI passes.

**Changing a rule.** Follow the invariants in the repo's `CLAUDE.md`:
update the rule, the example, `arch_test.go` and `.golangci.yml`
together, update the reason here, and walk the prompts in
`evals/go-architecture.json`.

## Known limits

- The example has no authentication, so idempotency keys are global.
  The docs say to scope them per caller once auth exists.
- Persistence guidance covers Postgres only, and the example's stores
  are in memory.
- HTTP and JSON APIs only; nothing on gRPC or message consumers beyond
  the outbox notes.
- The lint config hard-codes the module path `example.com/shop`;
  copies must change it. The architecture tests read it from `go.mod`.
- The architecture tests are tripwires based on names and syntax, not
  proofs; code review covers the rest.
