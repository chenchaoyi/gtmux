# supervisor-agent (delta)

## ADDED Requirements

### Requirement: The playbook teaches the consumption watermark

The seeded playbook SHALL teach the perception MODEL, not merely the new class, because the
change is to what HQ may assume about its own awareness. It SHALL state that:

- wake classes are PRIORITY labels — what to read first — and never the set of things HQ
  can know about;
- gtmux tracks HQ's consumption watermark and knocks `unread` for anything past it, so HQ
  never has to remember to poll: an unconsumed event knocks again, and keeps knocking,
  until it is read;
- an `unread` line carries a count and no importance claim, so HQ pulls and judges as
  usual, and a repeat of the same count means HQ did not actually consume;
- its own unfiltered `--since-seq` delta read IS the writeback, while a filtered read and a
  skip-ahead read are not, and `gtmux events --ack <seq>` is the explicit form for a stream
  reconciled another way.

Per-class instructions to remember to look for a particular missed trigger SHALL be
retired in favour of that general guarantee. The version SHALL be bumped so existing homes
adopt it.

#### Scenario: The playbook separates priority from coverage

- **WHEN** the HQ home is seeded or upgraded
- **THEN** the playbook states that a class says what to look at first rather than what
  exists, and that anything unconsumed re-knocks until read

#### Scenario: The playbook names what counts as consumption

- **WHEN** the playbook covers the wake→pull loop
- **THEN** it states that the unfiltered `--since-seq` read is the writeback, that a
  filtered or skip-ahead read is not, and that `gtmux events --ack` is the explicit form

### Requirement: HQ consumption lag is visible in doctor

`gtmux doctor` SHALL report, when an HQ home exists, how far HQ's consumption watermark is
behind the event stream and how long that backlog has been standing, flagging it as needing
attention past either threshold (20 events or 30 minutes). A watermark that has never been
recorded SHALL read as a neutral note rather than as a failure.

This exists because the wake channel is silent in both directions: a knock that lands
leaves no trace on any screen the user reads, and neither does one that never lands — so a
supervisor that has stopped consuming is otherwise indistinguishable from a fleet where
nothing has happened, and the only remaining detector is the user noticing that a finished
job went unremarked.

#### Scenario: A supervisor that stopped consuming is visible

- **WHEN** HQ has not advanced its watermark for longer than the standing threshold while
  events accrue
- **THEN** `gtmux doctor` reports the event-consumption row as needing attention, naming
  how far behind and for how long

#### Scenario: A normal in-flight delta is not a fault

- **WHEN** a handful of events have accrued in the last minute
- **THEN** the row reads OK — the backlog is what the knock is for

#### Scenario: A fresh HQ is not overdue

- **WHEN** an HQ home exists but HQ has never pulled the stream
- **THEN** the row is a neutral note rather than a warning
