# supervisor-agent (delta)

## ADDED Requirements

### Requirement: Capture is a verified step of the closed-loop turn

The seeded playbook SHALL upgrade HQ's closed-loop turn from `SENSE → JUDGE → REPORT`
to `SENSE → JUDGE → CAPTURE? → REPORT`, making knowledge capture a first-class step
rather than an out-of-loop good intention. A capture VERDICT SHALL be MANDATORY on
exactly three closure classes — `correction` (the commander corrects HQ), `crash` /
`StopFailure`, and `recurrence` (any footgun or fact hit a SECOND time) — because each is
a durable, cross-cutting lesson almost by definition. On a forced class the turn SHALL
emit exactly one verdict: either `⟣ 📓 captured: <topic-file>` (naming the knowledge-base
topic file it wrote/updated: accounts | workflows | best-practices | pitfalls |
corrections), OR an explicit "nothing durable" clause stating why the closure is not a
reusable cross-cutting fact. For `done` and `resolved` closures capture SHALL be
OPPORTUNISTIC with a SILENT default — HQ captures and marks a genuinely reusable fact if
one surfaced, but SHALL NOT be forced to emit a verdict, because forcing on those
high-frequency closures would degrade into ritual noise and pressure filler entries. The
capturable criterion SHALL be `reusable ∧ cross-cutting (across sessions / repos / tasks)
∧ not unique to this conversation`; pure this-task state (who is doing what, a specific PR
number) is board material, NOT a KB entry. Routine intermediate steps and non-closure
wakes (`tick`, `new-session`, `waiting`) SHALL NOT force a verdict. The shipped playbook
version SHALL be bumped so existing homes adopt the capture-verify on their next
managed-playbook upgrade.

#### Scenario: A commander correction forces a capture verdict

- **WHEN** HQ processes a `correction` closure
- **THEN** its turn emits either a `⟣ 📓 captured: <topic-file>` line naming the KB topic
  it wrote (typically `corrections`), or an explicit one-clause "nothing durable"
  judgment — it cannot close the correction without one

#### Scenario: A recurrence forces a capture verdict

- **WHEN** the same footgun or fact is hit a second time
- **THEN** the recurrence forces a capture verdict, because a repeat proves the fact is
  cross-cutting and was not yet captured

#### Scenario: A completed goal is opportunistic, not forced

- **WHEN** HQ processes a `done` or `resolved` closure
- **THEN** it captures and marks a reusable fact only if one genuinely surfaced, and is
  otherwise silent — no capture verdict is forced

#### Scenario: A board note never counts as capture

- **WHEN** HQ records a lesson only to the situation board
- **THEN** the playbook does not accept that as a capture verdict — the verdict must name
  a KB topic file (`⟣ 📓 captured: …`)

### Requirement: HQ consults the knowledge base as a hard precondition before advising or dispatching

The seeded playbook SHALL harden consultation from a soft suggestion into a HARD
precondition: before ADVISING the commander or DISPATCHING a task, HQ SHALL first consult
the relevant knowledge-base topic. When it advises, HQ SHOULD name the KB entry its advice
rests on. When NO KB entry covers the case, that gap SHALL itself be a capture trigger —
HQ captures the missing fact afterward so the next occurrence is covered. This
requirement SHALL NOT loosen the rule that HQ never answers another agent's
permission/plan/design choice.

#### Scenario: Advice is grounded in the base

- **WHEN** HQ gives the commander a recommendation about a repo or workflow the KB covers
- **THEN** it consulted the relevant topic first and names the entry its advice rests on

#### Scenario: A coverage gap becomes a capture

- **WHEN** HQ advises or dispatches on a matter no KB topic covers
- **THEN** it treats the gap as a capture trigger and records the fact afterward

### Requirement: The board and knowledge base have welded, non-interchangeable definitions

The seeded playbook SHALL weld the definitions so an ephemeral board note can never stand
in for durable capture. The SITUATION BOARD (`board.md`) SHALL be defined as HQ's
EPHEMERAL private posture — mode/source, priority, health, pending decisions, standing
context — which gtmux never reads back and HQ re-reads itself after a context reset. The
KNOWLEDGE BASE (`knowledge/`) SHALL be defined as the machine's DURABLE, cross-session,
reusable memory — accounts, workflows, best-practices, pitfalls, corrections. The playbook
SHALL state that the capture-verify routes a lesson ONLY into the KB, that "I noted the
board" can NEVER count as capture, and that the two may be written together but neither
substitutes for the other.

#### Scenario: The charter distinguishes ephemeral posture from durable memory

- **WHEN** the managed playbook is generated
- **THEN** it defines the board as ephemeral private posture and the KB as durable
  cross-session memory, and states that a board note is never a capture

### Requirement: gtmux capture records a distill candidate to the pending-distill spool

The system SHALL provide `gtmux capture "<one-line lesson> @<topic>"` (topic ∈ accounts |
workflows | best-practices | pitfalls | corrections) as a PUBLIC command so that HQ OR
ANY WORKER can record a durable-fact CANDIDATE cheaply in the moment, widening the capture
surface from a single supervisor to the whole fleet. It SHALL be safe to open this input
because a candidate is NOT a KB entry: the distillation pass (HQ's curation) is the
quality gate. The command SHALL append one JSON line — the lesson, the topic TAG, a DEDUP
KEY (topic + a lesson slug, or an explicit key), and AUTO-COLLECTED event context
(current/related `pane_id`, the current event `seq`, `task_id` if any, a timestamp) — to a
pending-distill spool under the HQ home
(`~/.config/gtmux/hq/knowledge/.pending-distill.jsonl` or the state-dir equivalent).
`gtmux capture --list` SHALL render the pending queue. A missing or invalid `@topic` SHALL
be an error. The distillation pass SHALL drain the spool MERGING each candidate by (topic,
dedup key) into an existing KB entry or an earlier same-key candidate rather than
manufacturing a near-duplicate, then truncate the spool. Being a public command it SHALL
be documented per the command-drift rule: the CLAUDE.md command list, a `docs/cli.md`
section, and `gtmux --help` (en+zh) — NOT the `check-design.sh` HIDDEN allowlist.

#### Scenario: A candidate is captured in one line with a dedup key

- **WHEN** any worker runs `gtmux capture "wrangler TLS-resets from the office; retry @pitfalls"`
- **THEN** one JSON line — lesson + topic tag `pitfalls` + a dedup key + auto-collected
  pane/seq/task/time context — is appended to the pending-distill spool

#### Scenario: An invalid topic errors

- **WHEN** `gtmux capture` is called with no `@topic` or an unknown topic
- **THEN** the command errors and writes nothing

#### Scenario: Distill merges same-key candidates rather than duplicating

- **WHEN** two candidates share a (topic, dedup key) and a distill pass drains the spool
- **THEN** they are merged into one KB entry (no near-duplicate) and the spool is truncated

### Requirement: HQ echoes matching knowledge at dispatch time

At `gtmux spawn` / dispatch time, the system SHALL auto-echo the pitfalls / workflows
knowledge-base summary that matches the target repository (by cwd repo name) and the goal
keywords, handing it to the worker at launch, so captured knowledge is surfaced at the
moment it is needed rather than left to HQ to recall each time. This is the tool-layer
mechanism that structurally closes "captured but never used" — the payoff of the capture
layers — and is a first-class deliverable. When no KB topic matches, the echo SHALL be a
no-op (nothing surfaced, no error).

#### Scenario: A dispatch surfaces the repo's known footguns

- **WHEN** HQ dispatches work into a repo whose `pitfalls`/`workflows` topics have entries
- **THEN** the matching KB summary is echoed to the worker at dispatch time

#### Scenario: No match is a silent no-op

- **WHEN** a dispatch targets a repo/goal with no matching KB entry
- **THEN** nothing is echoed and no error is raised

## MODIFIED Requirements

### Requirement: Signal register separates wake traffic from conversation

The seeded playbook SHALL mandate two output registers: replies to wake lines use
the SIGNAL register — one line opening with `⟣` and a fixed glyph vocabulary
(✅ done-judgment, ▪ noted-to-board, 📓 captured-to-KB, ◈ tick brief, ⚠ escalation),
with at most two indented detail lines (tick briefs ≤6 lines total) — while replies to the
human use ordinary conversational prose with no sigils. The `⟣ 📓 captured: <topic-file>`
line SHALL be the capture verdict named in the capture-verify requirement and SHALL be
emitted ONLY on a REAL capture (never as an empty "considered it" marker). Wake turns
SHALL be short: pull the delta, judge, capture (when a forced class or a real
opportunistic fact warrants it), update the board, emit the signal line; no narration.

#### Scenario: A done wake gets a one-line judgment

- **WHEN** HQ processes a `done` wake for a session that completed its goal
- **THEN** its reply is a single `⟣ ✅ …` line (judgment + suggested next step),
  visually distinct from conversation

#### Scenario: A capture is marked in the register

- **WHEN** HQ folds a durable lesson from a forced-class closure into a KB topic file
- **THEN** it emits a `⟣ 📓 captured: <topic-file>` line, in the signal register, never
  mixed with human prose

#### Scenario: A trivial done is noted silently

- **WHEN** HQ judges a done wake to be an unremarkable intermediate completion
- **THEN** it updates the board and replies with at most one `⟣ ▪` note line, and emits no
  `⟣ 📓` (capture is opportunistic on done, not forced)

### Requirement: gtmux raises a periodic knowledge-distillation trigger

The system SHALL raise a `distill` trigger to a live HQ when a knowledge-distillation
pass is due, decided LLM-free in the serve slow-tick — no LLM runs in the timing loop; HQ
performs the actual curation on the control record. The baseline trigger SHALL remain the
existing coarse cadence: a rate limit (at most one distill per configured minimum
interval), then an EVENT-VOLUME floor OR a WEEKLY time floor, with a ZERO-CHANGE gate
(nothing notable accrued → no trigger, no cost). The system MAY additionally fire on
EVENT-DRIVEN triggers, layered on top of that baseline and DEFERRED behind an observation
gate (built only if capture still slips once the mandatory capture-verify and the public
`gtmux capture` command are in use): (a) a DENSITY threshold of K notable CLOSURES accrued
since the watermark (K CONFIGURABLE — tunable without a release — default 10, range
10–12); (b) any COMMANDER CORRECTION (a `correction`-class event) in the delta, which
SHALL distill promptly while still respecting the minimum interval so a correction storm
cannot hammer the pane; and (c) the pending-distill SPOOL reaching N entries (N
CONFIGURABLE, default 5). Each raised trigger SHALL advance the `last-distill` watermark
(event sequence / timestamp marker) so the next pass distills only the DELTA. The
distillation pass SHALL additionally drain the pending-distill spool — MERGING each
candidate by (topic, dedup key) into the right KB entry rather than appending a
near-duplicate, and truncating the spool. When no HQ pane exists, no trigger SHALL be
raised.

#### Scenario: The periodic floor guarantees a pass on a quiet fleet

- **WHEN** the weekly/volume floor is reached with at least one notable event accrued and
  the rate limit has elapsed
- **THEN** gtmux raises exactly one `distill` control record and advances the watermark
  (the retained baseline)

#### Scenario: A density of closures triggers a distill (when the event-driven layer is built)

- **WHEN** the event-driven layer is enabled and at least K notable closures have accrued
  since the watermark and the rate limit has elapsed
- **THEN** gtmux raises one `distill` control record before the periodic floor would have
  fired

#### Scenario: A commander correction distills promptly (when the event-driven layer is built)

- **WHEN** the event-driven layer is enabled, a `correction`-class event enters the delta,
  and the minimum interval has elapsed
- **THEN** a `distill` trigger is raised promptly rather than waiting for the periodic floor

#### Scenario: Nothing to distill costs nothing

- **WHEN** a cadence boundary is reached but no notable event accrued since the last
  distill
- **THEN** no `distill` trigger is raised (the zero-change gate)

#### Scenario: No trigger without a supervisor

- **WHEN** no HQ pane is present
- **THEN** no `distill` trigger is raised
