# usage-watch — delta

## MODIFIED Requirements

### Requirement: Warnings reach the user and the supervisor

A breached or projected threshold SHALL surface as an amber usage MODIFIER on
the radar row (a modifier like errored/bg — never a status), and — when an hq
session is live — as one `usage·warn` WAKE (deduped per session+layer like the
waiting wake; `hqNudge:false` disables).

The wake SHALL ride the single wake channel like every other injection: the declared
`usage·warn` class, the `» gtmux·<class>` signal format, and the channel's draft guard,
ack, and queue. It SHALL NOT be hand-built and typed into the pane directly — that path
had no draft guard, so a warning firing while the user was mid-sentence in HQ appended
itself to their draft AND submitted it.

#### Scenario: Warn nudges the supervisor once

- **WHEN** a session first breaches (or projects into) a layer while HQ is live
- **THEN** one `usage·warn` wake reaches the HQ pane; an unchanged breach is not
  re-nudged

#### Scenario: The warning cannot clobber a half-typed HQ draft

- **WHEN** a usage warning fires while the user is composing in the HQ pane
- **THEN** nothing is typed: the wake queues and lands once the box is empty, like every
  other wake

### Requirement: Limits surface and warn

The system SHALL surface the windows in `gtmux usage`/`gtmux limits` (+ `--json`
and `GET /api/usage`), and SHALL raise a warning (the amber usage modifier +, when
HQ is live, one `limits·warn` wake through the wake channel, deduped per window) when a
window crosses its configured threshold (default: any weekly window ≥ 85%).

#### Scenario: Weekly window near the cap warns

- **WHEN** a weekly window reports ≥ the warn threshold
- **THEN** `gtmux limits` marks it and one `limits·warn` wake reaches a live HQ,
  at most once per window per crossing
