# usage-watch (delta)

## MODIFIED Requirements

### Requirement: Layered thresholds with ahead-of-time projection

The system SHALL evaluate each session against PER-AGENT-TYPE thresholds from
`~/.config/gtmux/usage.json` (sensible defaults when absent): context fraction,
per-session burn, and per-agent-type aggregate rate. It SHALL also
PROJECT (current + rate × horizon) and flag a session/type whose projection
crosses a threshold BEFORE it is reached. The first breached-or-projected layer
is reported as a compact `usage_warn` string.

A MONOTONIC CUMULATIVE quantity (per-session total burn, or any figure that only ever
grows) SHALL alarm on its RATE or WINDOWED DELTA, never on the total crossing a line: a
total-over-line alarm has no de-assert condition in physics, so it re-fires forever
(measured: `usage·warn burn` knocked three times in one round at 23.6M→23.7M→23.9M — the
"breach" can never clear while the session lives).

#### Scenario: Projected breach warns early

- **WHEN** a session's context is under the warn line but its rate projects
  crossing it within the horizon
- **THEN** its usage row carries a `usage_warn` naming the layer and the ETA

#### Scenario: Thresholds are per agent type

- **WHEN** usage.json sets different limits for claude vs codex
- **THEN** each session is judged against its own agent type's layers

#### Scenario: A growing total under a flat rate never alarms

- **WHEN** a session's cumulative burn keeps growing at an unremarkable, steady rate
- **THEN** no burn-layer `usage_warn` fires; a rate or windowed-delta spike still does

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

A queued `usage·warn` whose premise can be externally reset SHALL be re-sampled
immediately before delivery (the standing-knock rule). Context occupancy in particular
has a reset event neither gtmux nor its consumer controls — the harness's auto-compact —
so a ctx-layer warn delivered without revalidation can assert a state that no longer
exists (measured: a `ctx→80% in ~4m` projection warn arrived while the pane's live
status bar read 34%, the auto-compact having landed mid-flight). A warn whose re-sampled
value no longer breaches or projects SHALL NOT be delivered.

#### Scenario: Warn nudges the supervisor once

- **WHEN** a session first breaches (or projects into) a layer while HQ is live
- **THEN** one `usage·warn` wake reaches the HQ pane; an unchanged breach is not
  re-nudged

#### Scenario: The warning cannot clobber a half-typed HQ draft

- **WHEN** a usage warning fires while the user is composing in the HQ pane
- **THEN** nothing is typed: the wake queues and lands once the box is empty, like every
  other wake

#### Scenario: An auto-compacted ctx warn is not delivered stale

- **WHEN** a ctx-layer `usage·warn` is queued and the session's context is externally
  compacted before the queue flushes, so the re-sampled value no longer breaches or
  projects
- **THEN** the stale warn is not delivered
