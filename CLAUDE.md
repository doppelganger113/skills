# Working in this repository

This repo publishes Claude agent skills. Each skill is a folder under
`skills/` that gets installed as-is, so everything in it is written for
the agent. Humans read the root `README.md` and `docs/`.

## Layout

```
skills/<name>/           the skill; installed and zipped as-is
  SKILL.md               frontmatter (name, description) + instructions
  references/*.md        detail the agent reads on demand
  examples/              runnable examples the agent copies from
docs/<name>.md           why the skill says what it says (humans only)
evals/<name>.json        test prompts with expected behaviour
scripts/check_skills.py  structure checks, run in CI
.github/workflows/       skills.yml (all skills) + <name>.yml (per skill)
```

## Invariants

- Nothing human-facing inside `skills/<name>/`: no README, GUIDE,
  CHANGELOG or CONTRIBUTING files. They ship with the skill and the
  agent may follow them instead of SKILL.md. `scripts/check_skills.py`
  enforces this.
- `name` in SKILL.md frontmatter matches the folder name and doesn't
  contain "anthropic" or "claude". The
  description says what the skill does and when to use it, in at most
  1024 characters, with no angle brackets.
- Keep SKILL.md under about 500 lines. Detail goes in `references/`,
  one level deep, each file over about 100 lines starting with a
  Contents list. SKILL.md says when to read each reference.
- Rules change in one go: when a rule in SKILL.md or a reference
  changes, update the example, its lint config and its tests to match,
  and update the reasoning in `docs/<name>.md` if the reason changed.
  For go-architecture that means `examples/shop/arch_test.go` and
  `examples/shop/.golangci.yml`.
- `docs/<name>.md` explains reasons and links to SKILL.md sections. It
  never restates rules, so it can't drift out of sync with them.
- Skills never tell an agent to fetch remote content and follow it, or
  to download and run scripts. They are instructions an agent executes
  with the user's permissions.
- All rights reserved. Don't add a license or licensed third-party
  content.

## Before calling a change done

Run what CI runs and report anything you couldn't run:

```sh
python3 scripts/check_skills.py
cd skills/go-architecture/examples/shop
gofmt -l .          # must print nothing
go vet ./...
go test -race -count=1 ./...
golangci-lint run ./...
```

For changes to a skill's behaviour, also walk the prompts in
`evals/<name>.json` and say which expectations the change affects.

## Adding a skill

1. `skills/<name>/SKILL.md` with frontmatter; references and examples
   as needed.
2. A row in the README's skills table.
3. `docs/<name>.md` with the reasoning, and `evals/<name>.json` with
   test prompts.
4. If it has a runnable example, `.github/workflows/<name>.yml` modelled
   on `go-architecture.yml`, with actions pinned to commit SHAs.

## Commits

- Prefix with the skill name when a change is about one skill:
  `go-architecture: require Go 1.26`.
- The author is the owner's GitHub noreply identity, never a personal
  email address.
