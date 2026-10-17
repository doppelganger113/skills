# Static checks: finding, classing and running them

Contents
- Why this file exists
- Where commands may come from
- Command classes
- When each class may run
- How to run
- What to record

## Why this file exists

Deterministic tools find some bugs reliably and for free; the reviewer covers what they can't. But many "check" commands execute code the repository controls: a Makefile target, a package script, a test, a linter whose config is a script. In a repository you don't control, running them is running a stranger's program with the user's permissions. So commands are only ever found, never invented, and each one is classed before it runs.

## Where commands may come from

Use only commands from these sources, in this order. If none yields a command, record "no static checks found" and continue; never make one up.

1. **The overlay**, `.claude/qa.md`, under `## Commands`. Classed like any other command; the overlay cannot class its own commands.
2. **What the repository declares**: Makefile, justfile or Taskfile targets named like lint, vet, check, typecheck or static; `package.json` scripts; tool sections in `pyproject.toml`, `setup.cfg` or `tox.ini`; steps in CI workflow files. Running the target or script itself is class E, because it runs whatever the repository defines. If you have read the target and it is a thin wrapper around a single class S tool, you may run that tool directly instead, with the flags the target uses.
3. **Ecosystem defaults** from the matching ecosystem reference, and only if the binary is installed: check with `command -v <tool>`.

## Command classes

- **S, static.** Reads source and data-only config (YAML, TOML, JSON, INI), executes no code from the repository and needs no network. The ecosystem references list which commands are S and under what conditions.
- **E, executes repository code.** Tests, builds that run build scripts, code generation, Makefile, justfile or Taskfile targets, package scripts, tox or nox, and any tool that loads a config file written in a programming language (for example `eslint.config.js`, `conftest.py`, `setup.py`, `build.rs`, a Gradle build) or loads plugins the config names.
- **N, network.** Needs the network, such as vulnerability database lookups or schema downloads.
- **Never.** Installing or downloading dependencies or tools, setup or bootstrap scripts, anything that writes tracked files (formatters with write flags, `go generate`, codemods), migrations, and anything that connects to a database, cluster, queue or other shared system.

A command is E as well as anything else it is if the binary resolves to a path inside the repository (`node_modules/.bin`, `.venv`, `venv`, `vendor/bin`, `./bin`, `./tools`), because the repository controls that binary. If you can't tell which class a command is, it is E.

## When each class may run

| Class | Attended | Unattended |
|---|---|---|
| S | Run. | Run. |
| E | Only after the user agrees. Ask once, listing every E command with what it runs. | Only if the instructions for this run say the environment is a sandbox with no network and no credentials. Otherwise record it as not run. |
| N | Only after the user agrees, in the same question as E. | Only if the instructions for this run allow network access. Otherwise record it as not run. |
| Never | Never; list it if it would have helped. | Never. |

Where an E command may run, run it in a copy of the repository in a temporary directory outside it, without `.env` files or other credential files, with credential and cloud environment variables unset, and never against a shared database or service.

## How to run

- Run from the repository root (or the copy's root), each command with a timeout of about 5 minutes.
- Pass the offline settings the ecosystem reference gives, so a tool fails instead of downloading.
- A command that fails or times out is recorded, never fixed, retried with different flags, or worked around. If the code doesn't build, say so in the brief; the reviewer still reviews it statically.

## What to record

One line per command, for the brief and for the Checks run list shown to the user:

`<command> | class S/E/N | ran, exit <code> | skipped: <reason> | not found`

Under each command that ran and reported findings, add those findings trimmed to the relevant lines, at most about 40 lines per command. The reviewer treats them as already known and doesn't repeat them.
