# agent-skills

Skills for AI coding agents following my way of doing things.

Each skill is a folder under `skills/` with a `SKILL.md` that tells the
agent when to use it and how, plus optional reference files and
examples it reads on demand.

## Skills

| Skill | What it does |
|---|---|
| [go-architecture](skills/go-architecture/SKILL.md) | Architecture rules for Go code that stays clean as it grows from a small tool to a large multi-team codebase: structure in stages added only when a trigger fires, feature packages instead of MVC layers, enforced dependency boundaries, and a tested golden example with lint rules and architecture tests. |

## Install

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

For a single project, use that repo's `.claude/skills/` instead of
`~/.claude/skills/`.

**Claude apps (claude.ai, desktop):** zip the skill folder and upload
the zip as a custom skill in the app's settings.

```sh
cd skills/skills && zip -r go-architecture.zip go-architecture
```

## Quality checks

Skills with runnable examples are checked in CI on every change (see
`.github/workflows/`). For `go-architecture` that means `go vet`,
`go test -race` (including the architecture tests) and
`golangci-lint` with the example's own config, all at zero issues.

## License

Copyright © 2026 Marko Kovačević. All rights reserved.
