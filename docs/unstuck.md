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

**A skill writes the brief, and it is the preferred entry point.** The
agent sees only what the brief contains, and an ad-hoc brief is thin and
leans towards the caller's own theory. The skill makes the brief
complete every time, checks citations and caps delegations, so the
agent's description points callers to it. Models rarely judge themselves
stuck, so `/unstuck` is still the main way in during interactive work.

**Claude may invoke the skill too, including unattended.** The first
version was user-only (`disable-model-invocation`) and always ended at a
"wait for the user" step, so long-running or scheduled tasks either
couldn't use it or called the agent directly and lost the brief, the
citation checks and the delegation cap. Model invocation is limited by
the description to observable triggers (2+ failed attempts, oscillating
fixes), which keeps Opus calls rare.

**Unattended, it may look but not touch.** Candidates such as "subtract
a component", "delete the feature" or "query staging" become destructive
or outbound actions if an unattended run executes them, and the
agent's output is shaped by repository content that may be misleading
or injected. So without a user the skill runs only checks that can't
reach beyond the machine, leaves everything else to a human, and then
drops the problem for the rest of the run, so the task can't drift into
carrying out a candidate on its own initiative. See the unattended steps
in [SKILL.md](../skills/unstuck/SKILL.md). Existing test suites aren't
automatically safe: integration tests can reach networks, credentials
and shared databases.

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

**Project instructions stay loaded; conventions become questionable.**
Claude Code can start the agent without CLAUDE.md (`omitClaudeMd`),
which looked like the way to free it from the session's walls. Three
reviewers rejected that. Conventions such as "no interfaces until a
second consumer" exist to counter the model's own defaults, so without
them the agent proposes habits, not lateral ideas. The exclusion is also
partial (path-scoped and managed rules still load, older versions ignore
the key), the direct-call path would run with no rules at all, and
feasibility suffers because layout, commands and version floors live in
CLAUDE.md. The real wall was that loaded rules read as binding, so the
agent now sorts them itself: hard rules bind, conventions are
assumptions it may question openly, and anything unclear counts as hard
so a wrong call fails safe. Tone, formatting and reporting instructions
are set aside because they compete with the output format the caller
parses; commands, layout and security instructions stay because the
agent needs them to ground ideas and stay safe. Rules files the agent
opens itself can only add restrictions, so a planted "rule" can't grant
anything.

**Non-technical options compete on the same gates.** Changing the
requirement or accepting a constraint often resolves stuck problems, but
a mandatory slot invited token "ask the stakeholder" filler.

## Rejected approaches

- Technique quotas, technique labels in the output, random-entry
  objects and a mandatory wildcard: they produced creativity theatre.
- Asking "would the main agent have proposed this?": not judgeable.
- Excluding bugs whose cause isn't isolated: allowed instead, but only
  for diagnostic ideas (bisect, minimal repro, other observability).
- Starting the agent without CLAUDE.md (`omitClaudeMd`) and having the
  caller tag the rules in the brief: a conservative caller re-imports
  the same walls, and a wrong or missing tag fails open. Worth
  revisiting only if an A/B run on the evals shows more useful
  convention-breaking candidates with no extra hard-rule violations.
- Converting the agent into a `context: fork` skill: the brief must be
  written in the main conversation first, and an agent file states its
  tool allowlist and model directly.

## Known limits

- Claude Code only: claude.ai rejects `argument-hint` on upload, and has
  no subagents.
- "Internal, do not show" depends on the model complying.
- Unattended mode depends on the model recognising that no user can
  answer and on it classifying a test as read-only. Permission settings
  for the unattended run (no push, no write outside a scratch copy) are
  the real enforcement; the skill is a second layer.
- No eval results yet. The bar for keeping the pair: on 8 to 10 past
  stuck problems with known fixes, it beats the main agent alone on at
  least 3, with no fabricated citations.
