# unstuck and lateral-thinker: why they say what they say

For whoever edits these later, most likely their owner. The rules live
in [skills/unstuck/SKILL.md](../skills/unstuck/SKILL.md) and
[agents/lateral-thinker.md](../agents/lateral-thinker.md); this file
records only the reasons, so a future edit doesn't undo a decision
without meaning to. If a reason here stops being true, change the rule
and this file together.

The pair went through two review rounds by three reviewer agents (devil's
advocate, context engineer, senior engineer) before the first commit.

## Design decisions

**The thinking happens in a subagent, not a skill.** The value is a
fresh context that hasn't converged on the main agent's path. A skill
loads into the already-anchored conversation and inherits its framing.

**A user-invoked skill writes the brief.** Models rarely judge themselves
stuck; they retry variations, so automatic delegation seldom fires. The
user usually notices first. The agent also sees only what the brief
contains, and an ad-hoc brief is thin and leans towards the caller's own
theory. The skill makes the brief complete every time.
`disable-model-invocation` keeps the main agent from spending an Opus
call on its own initiative through the skill; the agent's description
still allows delegation when its triggers are visible.

**The brief withholds the favoured theory but keeps ruled-out
hypotheses.** Leading with the caller's theory pulls the subagent back
onto the same track. Eliminated hypotheses are evidence, though: without
them the agent re-proposes what was already ruled out.

**Untried obvious steps become the baseline, not banned.** After two
failed attempts the untried next step is often the answer. Banning it
pushes the agent towards clever but worse candidates, so a candidate
must clearly beat it instead.

**Selection gates are checkable, not "novel".** The agent can't predict
what the same model would have proposed. It can check whether an idea
changes a different thing from the banned list, respects the hard
constraints and has a cheap falsifier.

**A falsifier, not a proof.** Migration and scaling candidates can't be
proven in 30 minutes, but their riskiest assumption can usually be
attacked in that time.

**Verdict first, three candidates, 400 words.** The caller branches on
the first line. Four candidates with six fields didn't fit the budget,
and the end of a long output is what gets truncated.

**Evidence is cited and checked.** A `path:line` that doesn't say what
the agent claims destroys trust fastest. The agent may cite only lines
it read, runtime evidence is quoted from the brief, and the skill opens
every citation before the user sees it.

**Read-only tools, no web.** The agent's output feeds a main agent that
has write tools. Web access would add a prompt-injection path for no
real gain; the model knows enough for analogies. Reads are capped so a
vague brief doesn't turn into a repository sweep.

**Opus.** Smaller models collapse towards the obvious answer, which
defeats the purpose. The agent is called rarely and returns little.

**Non-technical options compete on the same gates.** Changing the
requirement or accepting a constraint often resolves stuck problems, but
a mandatory slot invited token "ask the stakeholder" filler.

## Rejected approaches

- Technique quotas, technique labels in the output, random-entry
  objects and a mandatory wildcard: they produced creativity theatre.
- Asking "would the main agent have proposed this?": not judgeable.
- Excluding bugs whose cause isn't isolated: allowed instead, but only
  for diagnostic ideas (bisect, minimal repro, other observability).
- Converting the agent into a `context: fork` skill: the brief must be
  written in the main conversation first, and an agent file states its
  tool allowlist and model directly.

## Known limits

- Claude Code only: claude.ai rejects `disable-model-invocation` and
  `argument-hint` on upload, and has no subagents.
- "Internal, do not show" depends on the model complying.
- No eval results yet. The bar for keeping the pair: on 8 to 10 past
  stuck problems with known fixes, it beats the main agent alone on at
  least 3, with no fabricated citations.
