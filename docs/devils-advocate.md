# devils-advocate and advocatus-diaboli: why they say what they say

For whoever edits these later, most likely their owner. The rules live
in [skills/devils-advocate/SKILL.md](../skills/devils-advocate/SKILL.md)
and [agents/advocatus-diaboli.md](../agents/advocatus-diaboli.md); this
file records only the reasons, so a future edit doesn't undo a decision
without meaning to. If a reason here stops being true, change the rule
and this file together.

The design rests on a research pass done in October 2026 over the
decision-making literature on devil's advocacy and dialectical inquiry,
intelligence and military red-teaming doctrine, and studies of LLM
critics, debate, self-critique and sycophancy. Its load-bearing claims
were checked against primary sources before the design was written.
Sources for the reasons below are listed at the end.

## What the role is

A devil's advocate is an assigned, temporary challenger who makes the
strongest evidence-based case against a dominant plan before anyone
commits to it. Two things separate it from a contrarian, a cynic, a
debater or a general critic: it is assigned, and its aim is a better
decision rather than winning. The CIA's tradecraft primer states the
consequence that matters most for an agent: a valid outcome is greater
confidence in the original judgment.

The name comes from the Promoter of the Faith in Catholic
canonizations, a mandatory procedural opponent whose presence was a
condition of validity. Two popular stories about it are wrong: Sixtus V
founded the Congregation of Rites in 1588 rather than the office in
1587, and the 1983 reform kept the office while making the process
collaborative. The lesson that does survive is that a challenger who
shares the proposer's stake in a positive outcome stops challenging.

## Design decisions

**The critique happens in a subagent, not in the main session.** Models
rarely find their own errors and prefer their own outputs, and
self-refinement amplifies that. A fresh context that never saw the
deliberation is the strongest-supported design choice in the whole
evidence base.

**The skill writes a neutral, de-owned brief.** Critique from LLMs turns
more positive when the user says they wrote or like the work, and
models seldom challenge the framing they are handed. Rephrasing a
confident statement as an open question beat an explicit
anti-sycophancy instruction in one study. So the brief drops authorship,
confidence and effort cues, presents the proposal's reasons as claims to
test, and ends with an open question. The caller's own list of
suspected weak spots is withheld because LLM reviewers anchor on the
limitations they are shown.

**HOLDS is a first-class verdict, and the prompt says why.** A reviewer
asked to find gaps usually reports some even when the work is sound,
and an LLM devil's advocate reduced over-reliance on AI advice at the
cost of more under-reliance on correct advice. In one human study,
devil's advocacy made no difference when the first plan was already
right. A challenger
that can't say "this holds" teaches people to ignore it.

**Objections must be grounded, decision-relevant and checkable, and are
capped at five.** LLM critiques contain more nitpicks and hallucinated
problems than human ones, longer critiques trade precision for recall,
and on real pull requests the best LLM review setups had low precision,
with missing context the largest source of false positives. In the
debiasing studies a single contradicting reason improved calibration,
and one study (author attribution uncertain) found that asking for ten
counterfactuals increased hindsight bias where asking for two did not. The filter, the cap and the 20-file
read budget aim at fewer, truer findings.

**Attack load-bearing assumptions, not details.** Intelligence doctrine
starts devil's advocacy by stating the mainline judgment and its
assumptions, then choosing the most vulnerable. In the management
studies, structured critique beat consensus on the quality of both
recommendations and assumptions, and dialectical inquiry, which builds a
counter-plan on opposite assumptions, beat plain critique on assumption
quality. That is why counter-plan mode exists.

**Steelman first.** Rapoport's rules ask for a restatement fair enough
that the other side would accept it before any rebuttal. It is not
tested for LLM critics, but two-sided debate helped judges find the
truth far more often than one-sided consultancy, and a devil's advocate
that only argues against is structurally a consultant. The steelman
puts the other side in front of the reader.

**Calm, objective tone and calibrated disagreement, no contrarian
persona.** Objective devil's advocacy outperformed "carping" advocacy,
which produced the fewest alternatives and the lowest quality. In
multi-agent debate, moderate disagreement beat being told to disagree
with everything. Red-teaming handbooks warn against an over-adversarial
tone.

**Each objection says LIKELY or POSSIBLE.** People discount dissent they
know is assigned and can end up more attached to their original view.
Labelling which objections are the agent's genuine assessment and which
are only the strongest case against is an attempt to keep the real
ones from being discounted along with the exploratory ones. It is not
tested for AI advocates.

**Only the agent's genuine concerns move the verdict.** If an
exploratory objection could force a revision, almost every sound plan
would come back marked for revision and the HOLDS design would be lost.
Turning an exploratory concern into a check keeps it from being lost
while letting evidence, not the advocate's stance, decide. Unattended,
no one is there to weigh an unsettled concern, so it errs toward
holding back.

**Alternatives go in as names and claims, not as the author's case.**
The counter-plan needs to know what was already considered, but the
rejection reasons are the producer's framing, so they are restated as
assumptions the agent may attack.

**A premortem is one of the agent's lenses, not a separate mode.**
Imagining that the plan has already failed generates more failure
reasons and lowered overconfidence more than a pro/con list, but no
study showed better decisions from it. Folding it into the attack step
keeps its benefit without another mode to choose. The often-quoted
claim that prospective hindsight improves accuracy by 30% overstates
the original study, which reported more reasons, not more accurate
ones.

**Citations are checked, and blocking Checks are run when they are
safe.** A cited line that doesn't say what the agent claims destroys
trust fastest. Running a read-only Check on each blocking objection
adds the external signal that self-critique lacks.

**The output is shown unchanged; the main agent disagrees in its own
sentences.** The main agent is anchored on the plan. If it could reword
or drop objections, the fresh context would be wasted.

**Disputes go to a fresh run with the counter-evidence, never to an
argument.** Models wrongly concede under "are you sure?" pushback, and
debating agents more often flip from right to wrong than the reverse.
In the main user study, though, the only LLM devil's advocate that
improved decisions was an interactive one, so the skill keeps one round of engagement: a fresh
brief that carries the new evidence, judged on that evidence alone.
Disagreement without new evidence doesn't trigger a re-run, so no one
can shop for a softer answer. This remedy is untested.

**Unattended, findings may only make the plan more conservative.** The
agent's output is shaped by repository content, which may be wrong or
planted. Letting it stop, delay or narrow an irreversible step is safe;
letting it add actions or reach external systems would turn a critique
into an unreviewed instruction. Holding back the costly step until a
human has read the verdict is the point of running the review before
committing. A half-finished migration is worse than either end state,
which is why the review comes before the first step that changes shared
state. A Check is text shaped by repository content, so it is treated
like any other untrusted suggestion before anything runs. Without a
user, the main agent is the producer and anchored on its plan, so it
may not argue the advocate down or re-review until a verdict releases
the step.

**Read-only tools, no web.** The agent's output feeds a main agent that
can write and run commands. Web access would add a prompt-injection path,
and the evidence the critique needs is in the repository and the brief.

**Opus.** An assumption, not a tested result: smaller models are
expected to collapse further toward the framing they are given, which
would defeat the purpose. The agent runs rarely and returns
little.

**Project instructions stay loaded but can only constrain.** Custom
subagents load CLAUDE.md by default. Hard rules (security, data,
deployment) are exactly what a proposal must satisfy, so the agent
keeps them as constraints. Tone and formatting instructions are set
aside because they compete with the output format the caller parses,
and rules files the agent opens itself can only add constraints, so a
planted file can't grant anything.

**Separate names for the skill and the agent.** The skill is
`devils-advocate` because that is what people ask for. The agent is
`advocatus-diaboli`, the office's Latin name, so that "delegate to the
devil's advocate" can't resolve to the skill invoking itself.

**Invoked selectively.** Devil's advocacy helped on hard, ill-structured
problems and did nothing or slowed things down on easy or additive ones.
The US intelligence community found that not every product suits it.
Model invocation is limited to costly or hard-to-reverse steps to keep
Opus calls rare and the advocate worth listening to.

## Rejected approaches

- A harsh or contrarian persona ("disagree with everything"): carping
  critique and forced disagreement both performed worse.
- Running the critique inline in the main context: self-critique is
  the weakest option in the evidence. It remains only as a labelled,
  opt-in fallback when the agent isn't installed.
- Multi-turn debate between the main agent and the advocate: debate
  doesn't reliably beat cheaper methods, and back-and-forth erodes
  correct positions.
- A separate premortem mode: one more choice for little gain; the
  premortem lives inside the attack step.
- Validating every objection with a second subagent or merging several
  critics: it improves recall and precision on code review, but doubles
  Opus cost on a skill already reserved for costly decisions. Revisit
  if logged results show many false BLOCKING objections; the cheaper
  read-only Check run by the main agent covers the most important ones.
- Naming the agent `devils-advocate` like the skill: see above.
- The "tenth man" rule: it comes from the novel *World War Z*. The real
  institution, an Israeli military-intelligence devil's advocate unit,
  shows that a dissenter is only as useful as the decision-makers'
  willingness to act on it, which is why the verdict and bottom line
  come first.

## Known limits

- Claude Code only: claude.ai rejects `argument-hint` on upload, and has
  no subagents.
- No study has tested an LLM devil's advocate on an individual
  engineer's software or design decisions; most human evidence comes
  from student lab groups and the LLM-critic evidence from benchmarks
  with ground truth.
- The same model family critiques the plan, so blind spots are shared.
  The skill is one source of dissent, not a replacement for tests or a
  human reviewer.
- "Internal, do not show" and the unattended rules depend on the model
  complying. Permission settings for unattended runs are the real
  enforcement.
- No eval results yet. The bar for keeping the pair: on 8 to 10 past
  decisions with known outcomes, its BLOCKING objections are real in
  most cases, it returns HOLDS on most of the decisions that went well,
  and it changes at least two decisions that went wrong.

## Sources

- Catholic Encyclopedia, "Advocatus Diaboli"; John Paul II,
  *Divinus Perfectionis Magister* (1983), vatican.va; Pew Research
  Center (2014) on canonization counts.
- CIA, *A Tradecraft Primer: Structured Analytic Techniques* (2009),
  pp. 17–18; UK Ministry of Defence, *Red Teaming Handbook*, 3rd ed.
  (2021), paras 6.7–6.10; Marchio, *Studies in Intelligence* (2023), on
  devil's advocacy after the 1973 war.
- Mason (1969), *Management Science*; Schweiger, Sandberg & Ragan
  (1986), *Academy of Management Journal*; Schwenk & Cosier (1980);
  Valacich & Schwenk (1995); Murrell, Stewart & Engel (1993).
- Nemeth, Connell, Rogers & Brown (2001), *Journal of Applied Social
  Psychology*; Nemeth, Brown & Rogers (2001), *European Journal of
  Social Psychology*; Schulz-Hardt, Jochims & Frey (2002);
  Schulz-Hardt et al. (2006); Greitemeyer et al. (2006).
- Lord, Lepper & Preston (1984); Koriat, Lichtenstein & Fischhoff
  (1980); Mitchell, Russo & Pennington (1989); Klein (2007), *Harvard
  Business Review*; Veinott, Klein & Wiggins (2010).
- Dennett, *Intuition Pumps* (2013), on Rapoport's rules.
- Chiang et al. (IUI 2024); Ma et al. (2024, arXiv 2403.01791); Lee
  et al. (CHI EA 2025).
- Huang et al. (ICLR 2024); Kamoi et al. (TACL 2024); Xu et al. (ACL
  2024); Panickssery, Bowman & Feng (NeurIPS 2024).
- Sharma et al. (ICLR 2024); Laban et al., FlipFlop (2023); Cheng et
  al., ELEPHANT (2025); Dubois et al., UK AI Security Institute (2026).
- Liang et al. (EMNLP 2024); Khan et al. (ICML 2024); Smit et al.
  (ICML 2024); Wynn, Satija & Hadfield (2025).
- McAleese et al., CriticGPT (2024); SWR-Bench (FSE 2026,
  arXiv 2509.01494); arXiv 2412.01708 on LLM peer reviewers.
- Claude Code documentation: best practices and subagents.
