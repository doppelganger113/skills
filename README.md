# agent-skills

Skills for AI coding agents following my way of doing things.

Each skill is a folder under `skills/` with a `SKILL.md` that tells the
agent when to use it and how, plus optional reference files and
examples it reads on demand. Some skills delegate to a Claude Code
subagent, which lives as a single file under `agents/`.

## Skills

| Skill | What it does |
|---|---|
| [go-architecture](skills/go-architecture/SKILL.md) ([design notes](docs/go-architecture.md)) | Architecture rules for Go code that stays clean as it grows from a small tool to a large multi-team codebase: structure in stages added only when a trigger fires, feature packages instead of MVC layers, enforced dependency boundaries, and a tested golden example with lint rules and architecture tests. |
| [devils-advocate](skills/devils-advocate/SKILL.md) ([design notes](docs/devils-advocate.md)) | `/devils-advocate`: before you commit to a costly or hard-to-reverse plan, design or decision, writes a neutral brief with no authorship or confidence cues, gets a fresh-context challenge from the `advocatus-diaboli` agent, checks every file and line it cites, and shows a verdict (HOLDS, REVISE, RECONSIDER, NEED_INFO) with at most five ranked, evidence-backed objections. Saying a plan holds is a normal outcome. Claude can also invoke it before the first shared-state step of a costly plan; unattended, it may only hold back or defer steps. |
| [unstuck](skills/unstuck/SKILL.md) ([design notes](docs/unstuck.md)) | `/unstuck`: when a session is stuck, writes a complete brief (goal, constraints, what was tried and ruled out, evidence, secrets redacted), gets a second opinion from the `lateral-thinker` agent, checks every file and line it cites, and changes nothing until you pick a candidate. Claude can also invoke it after repeated failures, including in unattended runs, where it asks nothing and runs only read-only or local, reversible checks. |
| [qa](skills/qa/SKILL.md) ([design notes](docs/qa.md)) | `/qa [plan\|code\|conformance]`: finds the major defects and unhandled edge cases in an implementation plan, in code or a diff, or in code measured against its plan, for any stack. Runs the project's own static checks first, only under strict rules about which commands may execute repository code, then gets a fresh-context review from the `qa-reviewer` agent, checks every file and line it cites, runs read-only checks on the serious findings, and shows a verdict (CLEAR, MINOR_ONLY, ISSUES, INCOMPLETE) with at most six findings ranked P0 to P2, each with a concrete trigger. Finding nothing major is a normal outcome. Per-ecosystem checklists cover Go, JavaScript and TypeScript, Python, SQL migrations and Kubernetes; a repository can tailor it with `.claude/qa.md`. Unattended, it only reports and may hold back publishing a change with a serious finding. |

## Agents

| Agent | What it does |
|---|---|
| [advocatus-diaboli](agents/advocatus-diaboli.md) ([design notes](docs/devils-advocate.md)) | Read-only, fresh-context devil's advocate: steelmans the proposal, ranks its load-bearing assumptions, attacks the weakest with a premortem and other lenses, and keeps only grounded, decision-relevant, checkable objections, each graded by severity and by whether it is a real concern or only the strongest case against. Used by `devils-advocate`. |
| [lateral-thinker](agents/lateral-thinker.md) ([design notes](docs/unstuck.md)) | Read-only, fresh-context subagent that finds non-obvious approaches to a stuck problem: bans what was already tried, generates and filters ideas, and returns a verdict plus up to three candidates, each with cited evidence, cost, reversibility and a cheap falsifying test. Used by `unstuck`. |
| [qa-reviewer](agents/qa-reviewer.md) ([design notes](docs/qa.md)) | Read-only, fresh-context QA reviewer: walks every boundary in scope against a failure-mode checklist and per-ecosystem bug patterns, keeps only findings with a concrete trigger it couldn't refute, grades them P0 to P2 with a confidence, and reports what it read and didn't. Used by `qa`. |

## Install

**unstuck** needs its agent too, and works in Claude Code only (the
Claude apps have no subagents):

```sh
git clone https://github.com/doppelganger113/skills.git
mkdir -p ~/.claude/skills ~/.claude/agents
cp -R skills/skills/unstuck ~/.claude/skills/
cp skills/agents/lateral-thinker.md ~/.claude/agents/
```

For one repository, copy them into that repo's `.claude/skills/` and
`.claude/agents/` instead and commit both.

**devils-advocate** likewise needs its agent, and works in Claude Code
only:

```sh
git clone https://github.com/doppelganger113/skills.git
mkdir -p ~/.claude/skills ~/.claude/agents
cp -R skills/skills/devils-advocate ~/.claude/skills/
cp skills/agents/advocatus-diaboli.md ~/.claude/agents/
```

Skip the clone if you already have it from above. For one repository,
copy them into that repo's `.claude/skills/` and `.claude/agents/`
instead. To update, remove the old `devils-advocate` folder (in
`~/.claude/skills/` or the repository's `.claude/skills/`) first, so the
new copy doesn't land inside the old one.

**qa** likewise needs its agent, and works in Claude Code only:

```sh
git clone https://github.com/doppelganger113/skills.git
mkdir -p ~/.claude/skills ~/.claude/agents
cp -R skills/skills/qa ~/.claude/skills/
cp skills/agents/qa-reviewer.md ~/.claude/agents/
```

Skip the clone if you already have it. For one repository, copy them
into that repo's `.claude/skills/` and `.claude/agents/` instead. To
update, remove the old `qa` folder first. To tailor qa for a
repository, ask Claude to set up qa there; it writes `.claude/qa.md`
(invariants, hot spots, commands, out-of-scope paths, accepted risks
with an expiry) for you to review and commit.

**Claude Code**, for all your projects:

```sh
git clone https://github.com/doppelganger113/skills.git
mkdir -p ~/.claude/skills
cp -R skills/skills/go-architecture ~/.claude/skills/
```

To update an existing install, remove the old copy first so the new one
doesn't land inside it:

```sh
rm -rf ~/.claude/skills/go-architecture
cp -R skills/skills/go-architecture ~/.claude/skills/
```

**Claude Code**, for one repository: copy a single skill into that
repo's `.claude/skills/` and commit it, so everyone who clones the repo
gets it. For example, `go-architecture` into a Go repository:

```sh
git clone --depth 1 https://github.com/doppelganger113/skills.git /tmp/agent-skills
cd /path/to/your-repo
mkdir -p .claude/skills
cp -R /tmp/agent-skills/skills/go-architecture .claude/skills/
git add .claude/skills/go-architecture
git commit -m "Add go-architecture skill"
```

Claude Code picks it up the next time it starts in that repo. To
update, run `rm -rf .claude/skills/go-architecture` before copying
again. The skill's example has its own `go.mod` and sits in a
directory Go tooling skips (names starting with `.`), so it doesn't
affect the repo's build, tests or lint.

**Claude apps (claude.ai, desktop):** zip the skill folder and upload
the zip as a custom skill in the app's settings.

```sh
cd skills/skills && zip -r go-architecture.zip go-architecture
```

## Quality checks

Every push and pull request runs `scripts/check_skills.py`, which checks
each skill's structure and frontmatter. Skills with runnable examples
are also checked in CI on every change (see `.github/workflows/`). For `go-architecture` that means `gofmt`,
`go vet`, `go fix -diff` (no outdated idioms) and `go test -race`
(including the architecture tests) on both
the oldest Go version in `go.mod` and the current stable release, plus
`golangci-lint` with the example's own config, all at zero issues.
Workflow actions are pinned to commit SHAs and kept current by
Dependabot. A weekly scheduled run also tests against the newest Go and
golangci-lint, so breakage shows up even when nothing changes here.

## License

Copyright © 2026 Marko Kovačević. All rights reserved.
