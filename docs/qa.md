# qa and qa-reviewer: why they say what they say

For whoever edits these later, most likely their owner. The rules live
in [skills/qa/SKILL.md](../skills/qa/SKILL.md), its
[references](../skills/qa/references/) and
[agents/qa-reviewer.md](../agents/qa-reviewer.md); this file records
only the reasons, so a future edit doesn't undo a decision without
meaning to. If a reason here stops being true, change the rule and this
file together.

The design came out of a design discussion in October 2026 between the
owner, a devil's advocate and a senior-engineer reviewer, not out of a
literature review. Where a reason rests on evidence, it is the evidence
already gathered for [devils-advocate](devils-advocate.md) on LLM
critics and self-review; claims specific to qa are marked as
assumptions.

## What it is, and what it isn't

qa finds what will break in a plan or in code before it ships:
defects, unhandled edge cases and failure modes, each with a concrete
trigger. devils-advocate decides whether a proposal should be committed
to at all and argues the strongest case against it. The two overlap on
plans, so each description excludes the other's job (the
devils-advocate description now points plan edge cases and code bug
hunts to qa): "will this
implementation work under failure, concurrency and real data?" is qa;
"should we do this, and is there a better option?" is devils-advocate.
The trigger evals test that boundary in both directions.

## Design decisions

**A fresh-context reviewer, not self-review.** The reasons are the same
as for devils-advocate: models rarely find their own errors, prefer
their own output, and soften critique when they know whose work it is.
A reviewer that saw the work being produced is grading itself. The
brief drops authorship, confidence and the caller's own suspicions for
the same reason; reviewers anchor on the weak spots they are shown.

**One skill with three modes, not two skills.** Plan review and code
review do different work, but they share the failure-mode checklist,
the evidence bar, the severity scale and the output, and keeping those
in one place matters more than sharper triggering. The costs of one
skill are real and are handled directly: an explicit mode argument
beats inference, an ambiguous input gets a question when someone can
answer, and the inferred mode is shown on the first line so a wrong
guess is visible. Conformance is its own mode because "does the code do
what the plan says" is a common question that neither plan nor code
review answers. If the trigger evals show the skill firing on the wrong
requests or picking the wrong mode on more than about one prompt in
ten, split it.

**One read-only reviewer agent, not two.** Two agents were planned so
plan review could have fewer permissions than code review. Moving every
command into the main session, behind the run rules, made both
reviewers read-only, and two agents with the same tools would only
duplicate the prompt. Keeping command execution out of the agent also
means its output, which repository content shapes, never directly runs
anything.

**Static checks first, and the reviewer never repeats them.** Linters,
type checkers, vet tools and race detectors find their bug classes
deterministically and cheaply. A model restating their findings adds
noise and cost; its value is in what they can't see, such as semantics,
interactions and operational failure.

**Commands are found, classed, then gated.** Most "check" commands run
code the repository controls: Makefile targets, package scripts, tests,
linters with script configs or plugins, binaries inside the repository.
In a repository the user doesn't control, that is running a stranger's
program with the user's permissions, and in an unattended run nobody is
watching. A per-agent tool allowlist doesn't contain this: allowing
`npm test` allows whatever `npm test` does. So commands are never
invented, the class depends on what a command executes rather than its
name, and anything that executes repository code or needs the network
waits for the user, or for a run that declares a sandbox. A command
counts as static only in the exact form a reference lists, because the
same tool becomes code execution with one flag or environment variable
(`go vet -vettool`, `GOFLAGS=-toolexec`, a lint config that loads
plugins), and those are exactly what a hostile overlay or Makefile
would add. Only the user's or the scheduled task's own prompt can
declare a sandbox, because project instructions from the repository
load into the session automatically and would otherwise unlock
themselves. See
[tooling.md](../skills/qa/references/tooling.md). Offline settings
(for Go, `GOTOOLCHAIN=local` and `GOPROXY=off`) exist because a
`toolchain` line in `go.mod` would otherwise download and run a
different toolchain.

**General core, per-ecosystem references.** The owner wanted qa usable
on any project and tailored later per repository. The procedure,
categories, evidence bar and severity scale don't depend on the
language; concrete bug patterns and tool commands do. Keeping the
latter in references loaded by marker files keeps the core short and
lets an ecosystem be added without touching it. The risk is generic
output when no reference applies, which is why the brief says which
references were used and the skill notes ecosystems that have none.
The first references cover the owner's own stack (Go, PostgreSQL,
Kubernetes and k3s, Node.js with Playwright, Python).

**The overlay is data with bounded effects.** Per-repository tailoring
belongs in the repository, but a file in the repository can be written
by an attacker. Rather than deciding whether an overlay is trustworthy,
which the agent can't do reliably, every overlay gets the same small,
bounded set of effects: it can add focus, invariants and candidate
commands and can narrow a whole-repository review, but can't suppress
a P0, unlock commands, or change the procedure. Accepted risks need a
reason and an expiry, and suppressed findings stay listed, so a
suppression can't silently hide a regression for good.

**Prove or drop, then verify.** LLM reviewers produce plausible but
wrong findings, and on real pull requests missing context is the
largest source of false positives. Requiring a concrete trigger, an
attempt to find the code that would prevent it, and a confidence that
says whether every step was read, cuts that at the source. The main
session then checks every citation and runs read-only Checks for P0
and P1, adding the external signal that self-critique lacks. Refuted
findings stay visible so the reviewer's error rate is visible too.

**Caps, no style, and CLEAR as a normal result.** A reviewer asked to
find problems finds some, and a flood of minor ones teaches people to
ignore the output. Six findings, four leads and a ban on style and
generic advice keep the review readable; linters own style. The
false-positive rate is the metric that decides whether the skill
stays.

**Severity by impact under realistic conditions.** P0 to P2 are
defined by consequence (data, security, availability, irreversibility)
rather than by category, so a race and a missing timeout are graded on
what they do, not on what they are.

**COVERAGE is mandatory.** On a large repository the reviewer samples.
Saying what was and wasn't read prevents a CLEAR verdict from implying
coverage it doesn't have.

**Unattended, findings only hold things back.** The output is shaped by
repository content, which may be wrong or planted. Letting a finding
stop a change from being published is safe; letting it edit code,
commit, or reach external systems would turn a review into an
unreviewed instruction. The same reasoning is spelled out for
devils-advocate.

**Read-only tools, no web, Opus.** The agent's output feeds a session
that can write and run commands, and the evidence it needs is in the
repository and the brief. Opus is an assumption carried over from
devils-advocate: smaller models are expected to drift toward the
framing they're given.

## Rejected approaches

- Two skills (plan review and code audit): see above; revisit if
  routing fails the evals.
- Two reviewer agents with different tool sets: see above.
- Giving the reviewer a shell to run tests or write a reproduction:
  repository content would then decide what runs, and the reviewer
  would grade its own repro.
- A stack-specific skill tuned only to Go: the owner wants it for any
  project; ecosystem references give the specificity instead.
- Trusting an overlay by repository owner or path: the agent can't
  verify ownership reliably, and bounded effects make the question
  unnecessary.
- A second reviewer pass to refute every finding: it doubles Opus cost
  on every run. The in-agent refute step and the main session's Checks
  cover the most important findings; revisit if logged results show
  many false P0 or P1 findings.

## Known limits

- Claude Code only: claude.ai rejects `argument-hint` on upload, and has
  no subagents.
- The same model family wrote much of the code it reviews, so blind
  spots are shared. qa is one reviewer, not a replacement for tests or
  a human review.
- "Internal, do not show", the command classes and the unattended rules
  depend on the model complying. Permission settings and a real sandbox
  for unattended runs are the actual enforcement.
- Untested: whether a headless run's permission settings let the
  reviewer read the skill's references and a diff file outside the
  working directory. Small diffs go inline and an unreadable diff file
  leads to one inline re-send, but unreadable references only weaken
  the review. Check with one headless run before relying on it.
- Ecosystem references are hand-written from general knowledge and
  will have gaps; version-dependent items name their versions so they
  can be checked.
- No eval results yet. The bar for keeping it, as the notes in
  [evals/qa.json](../evals/qa.json) describe: on seeded-bug cases
  across at least three ecosystems plus clean controls, it beats both a
  plain "review this critically" prompt and the static tools alone on
  P0 and P1 recall, with a false-positive rate on clean code low enough
  that its findings are worth reading.
