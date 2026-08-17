# hq-attention-system (delta)

## MODIFIED Requirements

### Requirement: Zero-loss cursor catch-up and gap detection

The pull read SHALL be seq-exact and self-auditing at the moment of consumption:
`gtmux events --since-seq <n>` SHALL return exactly the retained events with
sequence greater than the cursor, oldest first, and SHALL detect a gap — the
first returned event not being cursor+1, a hole inside the returned tail, or an
EMPTY tail while a positive cursor sits behind the sequence counter (the counter
survives the log files, so nothing-retained-yet-behind is the severest loss, not
proof of none) — every time it reads. On a gap it SHALL warn the reader (one
CRITICAL line, separate from the record output so `--json` consumers stay
parseable) to rebuild from a full `gtmux digest` snapshot and then write the
watermark back explicitly with `gtmux events --ack`, suggesting the sequence
COUNTER as the target (with an empty tail the retained maximum equals the
cursor — a no-op suggestion). A cursor of 0 ("everything retained") never
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

#### Scenario: An empty tail behind the counter is a gap, not silence

- **WHEN** the journal files are gone (cleanup, disk fault) while the sequence
  counter survives, and HQ pulls `--since-seq N` with 0 < N < the counter
- **THEN** the read warns exactly like any other gap — an empty result is not
  read as "nothing happened"
