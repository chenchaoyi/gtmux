# hq-attention-system (delta)

## MODIFIED Requirements

### Requirement: Zero-loss cursor catch-up and gap detection

The pull read SHALL be seq-exact and self-auditing at the moment of consumption:
`gtmux events --since-seq <n>` SHALL return exactly the retained events with
sequence greater than the cursor, oldest first, and SHALL detect a gap — the
first returned event not being cursor+1, or a hole inside the returned tail —
every time it reads. On a gap it SHALL warn the reader (one CRITICAL line,
separate from the record output so `--json` consumers stay parseable) to rebuild
from a full `gtmux digest` snapshot and then write the watermark back explicitly
with `gtmux events --ack <seq>`. A cursor of 0 ("everything retained") never
reports a leading gap.

**A gap read is NOT consumption.** A read that detected a gap SHALL NOT advance
the consumption watermark implicitly — the warning would otherwise get exactly
one chance to be seen before the debt vanished. The debt stands, every
subsequent pull re-warns, and the unread completeness net keeps knocking (a
tally whose only content is a gap still knocks, and is never auto-consumed as
echo), until the reader acks explicitly after rebuilding. The consumption row
`gtmux doctor` reports SHALL show a standing gap as slipped.

#### Scenario: Restart replays missed events

- **WHEN** HQ last acked sequence N and pulls `--since-seq N` while the journal
  has advanced to N+K
- **THEN** the read returns the events with sequence in (N, N+K] and none twice

#### Scenario: A gap triggers reconciliation

- **WHEN** HQ pulls `--since-seq N` and the first retained event after N is not
  N+1 (events rotated away), or the returned tail has a hole
- **THEN** the output carries one CRITICAL warning telling HQ to reconcile from
  a full digest snapshot and then `--ack` — at the moment of the read, not at
  some daemon's startup

#### Scenario: A contiguous read stays clean

- **WHEN** HQ pulls `--since-seq N` and the retained tail is contiguous from N+1
- **THEN** no gap warning is printed and the read counts as consumption

#### Scenario: A gap read leaves the watermark where it was

- **WHEN** HQ pulls `--since-seq N` and a gap is detected
- **THEN** the watermark does not move, the next pull warns again, and only an
  explicit `gtmux events --ack <seq>` (after the digest rebuild) settles the
  debt

#### Scenario: An echo-only gap still knocks

- **WHEN** everything retained past the watermark is HQ's own echo except that a
  sequence hole stands between the watermark and the retained tail
- **THEN** the unread sensor does not auto-advance the watermark and knocks,
  naming the gap and the rebuild-then-ack exit

### Requirement: Self-check triggers sensed by gtmux

The system SHALL sense, LLM-free in the slow-tick, when HQ should run a self-check and
raise a `self-check` trigger to HQ (delivered as a journal control record, not counted as
user-facing). A trigger SHALL be raised when: the machine has been idle ≥ ~2 h with no
CRITICAL/NORMAL surfaced AND ≥ ~12 h since the last self-check (the resting-user case); OR
a threshold trips (open ledger entries over a cap, the journal over its rotation ceiling,
or a consumption-watermark gap); OR a daily floor (≥ 24 h since the last self-check).
Triggers SHALL be rate-limited to at most one per hour.

#### Scenario: Resting-user idle trigger

- **WHEN** the machine has been idle ≥ ~2 h with nothing surfaced and it has been ≥ ~12 h
  since the last self-check
- **THEN** gtmux raises a self-check trigger to HQ

#### Scenario: Threshold trigger fires immediately

- **WHEN** the open ledger exceeds its cap or the journal exceeds its rotation ceiling
- **THEN** gtmux raises a self-check trigger without waiting for idle

#### Scenario: Rate limited

- **WHEN** conditions would raise a second self-check trigger within an hour of the last
- **THEN** no second trigger is raised until the hour has elapsed
