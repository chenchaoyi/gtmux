# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: The seeded playbook teaches the knowledge-distillation ritual

The seeded playbook SHALL teach HQ, on a gtmux-raised `distill` trigger — delivered as a
STANDING-priority wake line (`» gtmux·distill …`) and recorded as a
`[CONTROL gtmux:distill]` entry in the session-event stream — to run a retrospective
knowledge-distillation pass: read the fleet's event/outcome delta since the last distill,
drain the pending-distill spool CANDIDATE BY CANDIDATE (`gtmux knowledge add --capture
<key>` to accept one with its provenance, `gtmux knowledge dismiss --capture <key> --why`
to reject one with a trace), fold durable cross-cutting facts into the right topic
through the knowledge verbs (`add` for a new lesson, `supersede` for one that replaces an
existing entry — the mechanical form of update-over-append), PRUNE stale or dead entries
with `retire --why`, and migrate any legacy-file lesson the pass touches into the ledger.
The ritual SHALL be distinct from `self-check` (own-artifact health housekeeping) and
`tick` (the user-facing summary brief). HQ SHALL default to SILENT distillation, printing
a one-line brief ONLY when it made a real curation; a charter-level lesson SHALL still be
flagged for a seed/spec update rather than only noted locally; and the
never-store-secrets rule SHALL continue to apply. Because the trigger is also a stream
record, the playbook SHALL teach that a distill missed on the wake channel is recoverable
by PULL (`gtmux events --since-seq`) rather than lost. The shipped playbook version SHALL
be bumped so existing HQ homes adopt the ritual on their next managed-playbook upgrade.

#### Scenario: Silent when nothing durable accrued

- **WHEN** a `distill` trigger fires and the period's delta yields no durable new fact,
  no stale entry, and no duplicate to merge
- **THEN** HQ performs the pass and prints nothing

#### Scenario: A real distillation is briefed in one line

- **WHEN** a `distill` trigger fires and HQ folds a recurring cross-session fact into a
  topic through the knowledge verbs and retires a dead entry
- **THEN** HQ prints a single one-line brief of what it curated

#### Scenario: The distill delta is not a duplicate of moment-capture

- **WHEN** a durable fact was already captured in the knowledge base the moment it was
  learned
- **THEN** the distill pass SUPERSEDES that entry rather than adding a second copy,
  because it works the delta since the watermark and consolidates rather than
  re-summarizes

#### Scenario: Secrets are never distilled into the base

- **WHEN** the period's activity includes a password, token, or private key
- **THEN** the distillation records only IDs / methods / pointers, never the secret

#### Scenario: A missed knock is recoverable by pull

- **WHEN** a `distill` wake line does not reach the HQ pane (queue eviction, a wake
  outage) but the trigger was raised
- **THEN** the `[CONTROL gtmux:distill]` record is still in the event stream, so HQ's next
  delta pull surfaces the pending pass instead of it being silently lost

### Requirement: Capture is a verified step of the closed-loop turn

The seeded playbook SHALL upgrade HQ's closed-loop turn from `SENSE → JUDGE → REPORT`
to `SENSE → JUDGE → CAPTURE? → REPORT`, making knowledge capture a first-class step
rather than an out-of-loop good intention. A capture VERDICT SHALL be MANDATORY on
exactly three closure classes — `correction` (the commander corrects HQ), `crash` /
`StopFailure`, and `recurrence` (any footgun or fact hit a SECOND time) — because each is
a durable, cross-cutting lesson almost by definition. On a forced class the turn SHALL
emit exactly one verdict: either `⟣ 📓 captured: <topic>` (naming the knowledge topic
whose ledger it wrote through `gtmux knowledge add`/`supersede`: accounts | workflows |
best-practices | pitfalls | corrections), OR an explicit "nothing durable" clause stating
why the closure is not a reusable cross-cutting fact. For `done` and `resolved` closures
capture SHALL be OPPORTUNISTIC with a SILENT default — HQ captures and marks a genuinely
reusable fact if one surfaced, but SHALL NOT be forced to emit a verdict, because forcing
on those high-frequency closures would degrade into ritual noise and pressure filler
entries. The capturable criterion SHALL be `reusable ∧ cross-cutting (across sessions /
repos / tasks) ∧ not unique to this conversation`; pure this-task state (who is doing
what, a specific PR number) is board material, NOT a KB entry. Routine intermediate steps
and non-closure wakes (`tick`, `new-session`, `waiting`) SHALL NOT force a verdict. The
shipped playbook version SHALL be bumped so existing homes adopt the capture-verify on
their next managed-playbook upgrade.

#### Scenario: A commander correction forces a capture verdict

- **WHEN** HQ processes a `correction` closure
- **THEN** its turn emits either a `⟣ 📓 captured: <topic>` line naming the KB topic
  whose ledger it wrote (typically `corrections`), or an explicit one-clause "nothing
  durable" judgment — it cannot close the correction without one

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
  a KB topic written through the knowledge verbs (`⟣ 📓 captured: …`)

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
be an error. The spool SHALL drain candidate by candidate, never by blind truncation:
`gtmux knowledge add --capture <key>` consumes EVERY pending line sharing that key (the
merge of same-key candidates) into one entry that inherits their provenance, and
`gtmux knowledge dismiss --capture <key> --why` removes them with a journal trace — so an
accepted and a rejected candidate stop vanishing identically. Being a public command it
SHALL be documented per the command-drift rule: the CLAUDE.md command list, a
`docs/cli.md` section, and `gtmux --help` (en+zh) — NOT the `check-design.sh` HIDDEN
allowlist.

#### Scenario: A candidate is captured in one line with a dedup key

- **WHEN** any worker runs `gtmux capture "wrangler TLS-resets from the office; retry @pitfalls"`
- **THEN** one JSON line — lesson + topic tag `pitfalls` + a dedup key + auto-collected
  pane/seq/task/time context — is appended to the pending-distill spool

#### Scenario: An invalid topic errors

- **WHEN** `gtmux capture` is called with no `@topic` or an unknown topic
- **THEN** the command errors and writes nothing

#### Scenario: Distill merges same-key candidates rather than duplicating

- **WHEN** two candidates share a (topic, dedup key) and a distill pass drains the spool
- **THEN** one `gtmux knowledge add --capture <key>` consumes both into ONE entry (no
  near-duplicate), the entry's provenance carries both sequences, and the remaining
  unrelated candidates stay pending
