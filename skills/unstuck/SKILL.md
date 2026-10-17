---
name: unstuck
description: Get a fresh-context second opinion from the lateral-thinker subagent on a problem the session is stuck on. Writes a complete brief from the conversation, delegates, checks the cited evidence and waits for the user to choose. Invoked by the user as /unstuck.
argument-hint: "[the problem, if not obvious from the conversation]"
disable-model-invocation: true
---

We are stuck. Get a second opinion from the `lateral-thinker` subagent.

Problem focus (may be empty): $ARGUMENTS

If no `lateral-thinker` subagent is available, say it must be installed into `.claude/agents/` and stop.

1. Write the brief from this conversation and the problem focus:
   - **Goal:** what "done" looks like.
   - **Hard constraints** vs **preferences**.
   - **Tried:** every approach so far in chronological order and neutral wording, each with the exact error, output or measurement it produced; or "none yet".
   - **Ruled out:** hypotheses already eliminated, each with the observation that ruled it out.
   - **Evidence:** relevant file paths; verbatim output of the last failing command, trimmed to the relevant lines; any pprof, kubectl or CI excerpt. Redact secrets, tokens and environment values.
   - **Earlier lateral-thinker output**, if any in this conversation.
   Leave out only the theory or approach you currently favour; keep it in mind for step 5.
2. If the goal cannot be filled from the conversation or the problem focus, ask me once with this template, then continue: Goal / Tried (approach and result, or none) / Hard constraints.
3. Delegate to the `lateral-thinker` subagent with the full brief.
4. If it replies `NEED: ...`, ask me for exactly those items, then re-send the full brief with them added. Never delegate a third time.
5. Before showing the output, open every `path:line` it cites and append `[CHECKED]` or `[MISMATCH: <what the line actually says>]`. Then show the output, followed by at most two sentences: whether you agree with its FIRST STEP and why, and whether any candidate matches the approach you had favoured.
6. Do not run any candidate's test or change any file until I choose one.
