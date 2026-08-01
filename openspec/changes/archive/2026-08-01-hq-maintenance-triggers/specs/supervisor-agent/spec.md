# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: gtmux raises a periodic knowledge-distillation trigger

The system SHALL raise a `distill` trigger to a live HQ when a knowledge-distillation
pass is due, decided LLM-free in the serve slow-tick — no LLM runs in the timing loop; HQ
performs the actual curation on the raised trigger. The baseline trigger SHALL remain the
existing coarse cadence: a rate limit (at most one distill per configured minimum
interval), then an EVENT-VOLUME floor OR a WEEKLY time floor, with a ZERO-CHANGE gate
(nothing accrued → no trigger, no cost). The system SHALL additionally fire on the
pending-distill SPOOL reaching N entries (N CONFIGURABLE, default 5), still behind the
rate limit, so a captured lesson reaches the knowledge base without waiting out the week;
a non-empty spool SHALL also satisfy the zero-change gate. The system MAY additionally
fire on further EVENT-DRIVEN triggers, DEFERRED: (a) a DENSITY threshold of K notable
CLOSURES accrued since the watermark (K CONFIGURABLE, default 10, range 10–12); and (b)
any COMMANDER CORRECTION (a `correction`-class event) in the delta.

The trigger's own control record SHALL NOT count as fleet activity in any sensor's input:
gtmux-authored control records SHALL be excluded when counting the accrued delta, so a
raised trigger can never satisfy its own zero-change gate. Each raised trigger SHALL
advance the `last-distill` watermark (event sequence / timestamp marker) so the next pass
distills only the DELTA. The distillation pass SHALL additionally drain the
pending-distill spool — MERGING each candidate by (topic, dedup key) into the right KB
entry rather than appending a near-duplicate, and truncating the spool. When no HQ pane
exists, no trigger SHALL be raised.

#### Scenario: A weekly pass is due

- **WHEN** the time floor has elapsed since the last distill and at least one notable
  event has accrued
- **THEN** gtmux raises exactly one `distill` trigger and advances the watermark

#### Scenario: A busy fleet distills before the log rotates

- **WHEN** the event-volume floor is reached well before the weekly floor (a high-churn
  period)
- **THEN** the `distill` trigger fires on the volume floor, so the delta is distilled
  before the size-bounded event log rotates it away

#### Scenario: A filled capture spool distills without waiting for the week

- **WHEN** the pending-distill spool holds at least N candidates and the rate limit has
  elapsed, but neither the weekly nor the volume floor has been reached
- **THEN** gtmux raises one `distill` trigger, so captured lessons are filed in days
  rather than at the next weekly floor

#### Scenario: A density of closures triggers a distill (when the event-driven layer is built)

- **WHEN** the event-driven layer is enabled and at least K notable closures have accrued
  since the watermark and the rate limit has elapsed
- **THEN** gtmux raises one `distill` trigger before the periodic floor would have fired

#### Scenario: A commander correction distills promptly (when the event-driven layer is built)

- **WHEN** the event-driven layer is enabled, a `correction`-class event enters the delta,
  and the minimum interval has elapsed
- **THEN** a `distill` trigger is raised promptly rather than waiting for the periodic floor

#### Scenario: Nothing to distill costs nothing

- **WHEN** a cadence boundary is reached but no event accrued since the last distill and
  the capture spool is empty
- **THEN** no `distill` trigger is raised (the zero-change gate)

#### Scenario: A raised trigger does not feed its own gate

- **WHEN** a `distill` trigger has been raised and recorded, and the next cadence boundary
  arrives with no other activity
- **THEN** the zero-change gate still holds — gtmux's own control record is not counted as
  accrued fleet activity

#### Scenario: No trigger without a supervisor

- **WHEN** no HQ pane is present
- **THEN** no `distill` trigger is raised

### Requirement: The seeded playbook teaches the knowledge-distillation ritual

The seeded playbook SHALL teach HQ, on a gtmux-raised `distill` trigger — delivered as a
STANDING-priority wake line (`» gtmux·distill …`) and recorded as a
`[CONTROL gtmux:distill]` entry in the session-event stream — to run a retrospective
knowledge-distillation pass: read the fleet's event/outcome delta since the last distill,
drain the pending-distill spool, fold durable cross-cutting facts into the right
knowledge-base topic file (preferring to UPDATE existing entries over appending
duplicates), and PRUNE stale or dead entries and merge duplicates — using only its
existing write-own-notes authority. The ritual SHALL be distinct from `self-check`
(own-artifact health housekeeping) and `tick` (the user-facing summary brief). HQ SHALL
default to SILENT distillation, printing a one-line brief ONLY when it made a real
curation; a charter-level lesson SHALL still be flagged for a seed/spec update rather than
only noted locally; and the never-store-secrets rule SHALL continue to apply. Because the
trigger is also a stream record, the playbook SHALL teach that a distill missed on the
wake channel is recoverable by PULL (`gtmux events --since-seq`) rather than lost. The
shipped playbook version SHALL be bumped so existing HQ homes adopt the ritual on their
next managed-playbook upgrade.

#### Scenario: Silent when nothing durable accrued

- **WHEN** a `distill` trigger fires and the period's delta yields no durable new fact,
  no stale entry, and no duplicate to merge
- **THEN** HQ performs the pass and prints nothing

#### Scenario: A real distillation is briefed in one line

- **WHEN** a `distill` trigger fires and HQ folds a recurring cross-session fact into a
  topic file and prunes a dead entry
- **THEN** HQ prints a single one-line brief of what it curated

#### Scenario: The distill delta is not a duplicate of moment-capture

- **WHEN** a durable fact was already captured in the knowledge base the moment it was
  learned
- **THEN** the distill pass updates that entry in place rather than appending a second
  copy, because it works the delta since the watermark and consolidates rather than
  re-summarizes

#### Scenario: Secrets are never distilled into the base

- **WHEN** the period's activity includes a password, token, or private key
- **THEN** the distillation records only IDs / methods / pointers, never the secret

#### Scenario: A missed knock is recoverable by pull

- **WHEN** a `distill` wake line does not reach the HQ pane (queue eviction, a wake
  outage) but the trigger was raised
- **THEN** the `[CONTROL gtmux:distill]` record is still in the event stream, so HQ's next
  delta pull surfaces the pending pass instead of it being silently lost

### Requirement: HQ self-check and self-maintenance

The seeded playbook SHALL teach HQ, on a gtmux-raised self-check trigger — delivered as a
STANDING-priority wake line (`» gtmux·self-check …`) and recorded as a
`[CONTROL gtmux:self-check]` entry in the session-event stream — to review and maintain
its OWN artifacts: event-log/feed health, attention-ledger archival and de-duplication,
memory/knowledge-base quality, and accumulated low-value items, using only its existing
write-own-notes authority. HQ SHALL default to SILENT self-maintenance, printing a
one-line brief ONLY when it took a real action, and SHALL escalate a severe finding
(rotation broken, cursor gap, mass-invalid memory) as CRITICAL. The self-check sensor's
own control records SHALL NOT be counted as recent user-facing attention, so a raised
trigger cannot suppress the idle condition that raised it.

#### Scenario: Silent maintenance when nothing needed

- **WHEN** a self-check trigger fires and HQ finds nothing to fix
- **THEN** HQ performs the pass and prints nothing

#### Scenario: A real cleanup is briefed in one line

- **WHEN** a self-check trigger fires and HQ archives closed ledger items or prunes stale
  memory
- **THEN** HQ prints a single one-line brief of what it did

#### Scenario: A severe finding escalates

- **WHEN** a self-check finds a broken rotation, a cursor gap, or mass-invalid memory
- **THEN** HQ surfaces it as CRITICAL rather than quietly cleaning up

#### Scenario: A raised self-check does not suppress the next one

- **WHEN** a self-check trigger has been raised into an otherwise quiet fleet and the idle
  window elapses again
- **THEN** the idle trigger still fires — gtmux's own control record is not read as the
  user having been recently pinged

## ADDED Requirements

### Requirement: Maintenance triggers are auditable from the event stream

Every gtmux-raised HQ maintenance trigger (`distill`, `self-check`) SHALL be appended to
the session-event journal as a control record carrying its event name, a human summary and
a severity, so that `gtmux events` answers "when did this last run?" without reading the
feed spool. The perception feed daemon SHALL spool these records on its normal journal-tail
path rather than receiving a separately written copy, so exactly one record exists per
raised trigger. The shared event formatter SHALL render a control record legibly (its
control tag, event name and summary) rather than as an empty lifecycle row.

#### Scenario: A raised trigger is queryable after the fact

- **WHEN** a user or HQ runs `gtmux events` over a window in which a distill pass was
  raised
- **THEN** the output contains a legible `[CONTROL gtmux:distill]` line with its reason,
  so "did the periodic pass run?" is answerable directly from the stream

#### Scenario: One record, not two

- **WHEN** a maintenance trigger is raised while the feed daemon is healthy
- **THEN** the record appears exactly once in the feed spool — appended to the journal and
  spooled by the daemon's tail, not written to both by hand

### Requirement: Maintenance staleness is visible in doctor

`gtmux doctor` SHALL report, when an HQ home exists, when the periodic distill and
self-check passes last ran and whether either has SLIPPED — that is, exceeded its own
cadence floor plus a grace window. A pass that has never run SHALL be reported as such
rather than as overdue. `gtmux capture --list` SHALL additionally head the pending-distill
queue with the same last-distill age, since that queue is what a distill pass drains.

#### Scenario: A slipped distill is visible at a glance

- **WHEN** no distill pass has run for longer than its weekly floor plus the grace window
- **THEN** `gtmux doctor` reports the distill row as needing attention, with the age of the
  last pass

#### Scenario: A healthy cadence reads as healthy

- **WHEN** the last distill is within its floor
- **THEN** `gtmux doctor` reports the row as OK with the age of the last pass

#### Scenario: A never-run pass is not an error

- **WHEN** an HQ home exists but no distill has ever been raised
- **THEN** the row reports "never run" as a neutral note rather than a failure
