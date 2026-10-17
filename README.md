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
| [unstuck](skills/unstuck/SKILL.md) ([design notes](docs/unstuck.md)) | `/unstuck`: when a session is stuck, writes a complete brief (goal, constraints, what was tried and ruled out, evidence, secrets redacted), gets a second opinion from the `lateral-thinker` agent, checks every file and line it cites, and changes nothing until you pick a candidate. Claude can also invoke it after repeated failures, including in unattended runs, where it asks nothing and runs only read-only or local, reversible checks. |

## Agents

| Agent | What it does |
|---|---|
| [lateral-thinker](agents/lateral-thinker.md) ([design notes](docs/unstuck.md)) | Read-only, fresh-context subagent that finds non-obvious approaches to a stuck problem: bans what was already tried, generates and filters ideas, and returns a verdict plus up to three candidates, each with cited evidence, cost, reversibility and a cheap falsifying test. Used by `unstuck`. |

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
