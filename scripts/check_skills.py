#!/usr/bin/env python3
"""Repository checks for the skills in skills/ and the agents in agents/.

Run from the repository root: python3 scripts/check_skills.py
Standard library only, so it runs on any CI runner without installs.

Checks, per skill folder:
  - SKILL.md exists and starts with frontmatter holding `name` and
    `description`; name is kebab-case, at most 64 characters, and
    matches the folder name and avoids the reserved words "anthropic"
    and "claude"; description is non-empty, at most 1024 characters,
    with no angle brackets; unquoted values are valid YAML plain scalars.
  - No human-facing docs (README, GUIDE, CHANGELOG, CONTRIBUTING)
    anywhere inside the folder: everything in a skill folder is
    installed with the skill and may be read by the agent, so human
    docs belong in docs/ instead.
  - The root README.md lists the skill.

Checks, per agent file in agents/:
  - Frontmatter with `name` (kebab-case, matching the file name),
    a non-empty `description` and an explicit `tools` line, so no agent
    silently inherits every tool.
  - The root README.md lists the agent.
"""

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
SKILLS = ROOT / "skills"
AGENTS = ROOT / "agents"
# Doc-like extensions only, so source files such as guide.go aren't flagged.
HUMAN_DOC = re.compile(
    r"^(readme|guide|changelog|contributing)(\.(md|markdown|txt|rst|adoc|org|html?))?$",
    re.IGNORECASE,
)
RESERVED = ("anthropic", "claude")
# A plain (unquoted) YAML scalar can't contain ": " or " #" or start with
# an indicator character; real YAML parsers reject or truncate it.
PLAIN_UNSAFE = re.compile(r": | #|:$|^[,\[\]{}#&*!|>'\"%@`]|^[-?:](\s|$)")
NAME = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")


def frontmatter(text):
    """Return the frontmatter's top-level key/value pairs, or None."""
    if not text.startswith("---\n"):
        return None
    end = text.find("\n---", 3)
    if end == -1:
        return None
    fields, key, problems = {}, None, []
    for line in text[4:end].splitlines():
        m = re.match(r"^([A-Za-z_-]+):\s*(.*)$", line)
        if m:
            key, value = m.group(1), m.group(2).strip()
            if value in (">", ">-", "|", "|-"):
                # Block scalars (description: >) continue on indented lines.
                fields[key] = ""
            elif len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
                fields[key] = value[1:-1]
            else:
                if PLAIN_UNSAFE.search(value):
                    problems.append(key)
                fields[key] = value
        elif key and line.startswith((" ", "\t")):
            fields[key] = (fields[key] + " " + line.strip()).strip()
    fields["__unsafe__"] = problems
    return fields


def check_skill(folder, readme_text):
    errors = []
    rel = folder.relative_to(ROOT)
    skill_md = folder / "SKILL.md"
    if not skill_md.is_file():
        return [f"{rel}: missing SKILL.md"]

    fm = frontmatter(skill_md.read_text(encoding="utf-8"))
    if fm is None:
        errors.append(f"{rel}/SKILL.md: missing or unterminated frontmatter")
    else:
        name = fm.get("name", "")
        desc = fm.get("description", "")
        for key in fm["__unsafe__"]:
            errors.append(
                f"{rel}/SKILL.md: {key} is not valid as an unquoted YAML value "
                f"(contains ': ' or ' #', or starts with a special character); "
                f"quote it or use a block scalar (>)"
            )
        if not NAME.match(name) or len(name) > 64:
            errors.append(f"{rel}/SKILL.md: name {name!r} must be kebab-case, at most 64 characters")
        if any(word in name for word in RESERVED):
            errors.append(f"{rel}/SKILL.md: name {name!r} must not contain 'anthropic' or 'claude'")
        if name != folder.name:
            errors.append(f"{rel}/SKILL.md: name {name!r} must match folder name {folder.name!r}")
        if not desc:
            errors.append(f"{rel}/SKILL.md: description is empty")
        elif len(desc) > 1024:
            errors.append(f"{rel}/SKILL.md: description is {len(desc)} characters, limit 1024")
        if "<" in desc or ">" in desc:
            errors.append(f"{rel}/SKILL.md: description must not contain angle brackets")

    for path in folder.rglob("*"):
        if path.is_file() and HUMAN_DOC.match(path.name):
            errors.append(
                f"{path.relative_to(ROOT)}: human docs don't belong in a skill folder "
                f"(it ships with the skill); put them in docs/{folder.name}.md"
            )

    if f"skills/{folder.name}/" not in readme_text:
        errors.append(f"README.md: add {folder.name} to the skills table")
    return errors


def check_agent(path, readme_text):
    errors = []
    rel = path.relative_to(ROOT)
    fm = frontmatter(path.read_text(encoding="utf-8"))
    if fm is None:
        return [f"{rel}: missing or unterminated frontmatter"]
    for key in fm["__unsafe__"]:
        errors.append(
            f"{rel}: {key} is not valid as an unquoted YAML value; "
            f"quote it or use a block scalar (>)"
        )
    name = fm.get("name", "")
    if not NAME.match(name) or len(name) > 64:
        errors.append(f"{rel}: name {name!r} must be kebab-case, at most 64 characters")
    if name != path.stem:
        errors.append(f"{rel}: name {name!r} must match file name {path.stem!r}")
    if not fm.get("description"):
        errors.append(f"{rel}: description is empty")
    if not fm.get("tools"):
        errors.append(f"{rel}: list the agent's tools explicitly (least privilege)")
    if f"agents/{path.name}" not in readme_text:
        errors.append(f"README.md: add {path.stem} to the agents table")
    return errors


def main():
    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    folders = sorted(p for p in SKILLS.iterdir() if p.is_dir())
    if not folders:
        print("no skills found under skills/", file=sys.stderr)
        return 1
    agents = sorted(AGENTS.glob("*.md")) if AGENTS.is_dir() else []
    errors = [e for f in folders for e in check_skill(f, readme)]
    errors += [e for a in agents for e in check_agent(a, readme)]
    for e in errors:
        print(f"error: {e}", file=sys.stderr)
    if errors:
        return 1
    print(f"ok: {len(folders)} skill(s) and {len(agents)} agent(s) checked")
    return 0


if __name__ == "__main__":
    sys.exit(main())
