# session-events Specification

## ADDED Requirements

### Requirement: Events carry a deterministic severity tier

Every event record SHALL carry an additive `severity` field classifying the event's
attention level as `routine`, `notable`, or `important`, computed by a DETERMINISTIC,
LLM-free classifier from fields the record already holds (event/state/kind/class) and
stamped at the SOURCE (the single append path), so it is persisted and queryable without
recompute. A `Waiting` event (the pane needs the user) and a `Stop` classified `asking`
(a reply-text question) SHALL be `important`; a `Stop` classified `report` and the session
lifecycle events (`SessionStart`/`SessionEnd`/`Resumed`/`PreCompact`) SHALL be `notable`;
prompt submissions, notifications, and ordinary working ticks SHALL be `routine`. The field
is additive to the stable event contract — a record without it (a legacy line) SHALL read as
`routine`. Stamping SHALL NOT alter the existing marker/notify state machines, and the write
path SHALL remain fire-and-forget so a busy or absent consumer never blocks the hook.

#### Scenario: A waiting event is important

- **WHEN** the hook appends a `Waiting` event (the pane blocked on the user)
- **THEN** the record's `severity` is `important`

#### Scenario: An asking turn-end is important, a report turn-end is notable

- **WHEN** a `Stop` event is classified `asking` versus `report`
- **THEN** the former's `severity` is `important` and the latter's is `notable`

#### Scenario: Routine chatter is routine

- **WHEN** a `UserPromptSubmit` (or other non-attention lifecycle tick) is appended
- **THEN** its `severity` is `routine`

#### Scenario: A legacy record without severity reads as routine

- **WHEN** a record written before this field is read back
- **THEN** it is treated as `routine` for severity purposes, without failing the read

### Requirement: Severity-filtered event read

`gtmux events` SHALL accept `--severity <level>` (`routine`|`notable`|`important`) to
restrict the stream to events at that level AND ABOVE (`routine` < `notable` < `important`),
applied to BOTH the bare recent-window form and `--follow`, so a supervisor reads the
attention stream — not every raw line — and, together with the per-source `summary` already
on each record, never needs to read a raw transcript to triage. An unrecognized level SHALL
be rejected with the usage message.

#### Scenario: Filter to attention-worthy events

- **WHEN** `gtmux events --severity important` runs over a stream mixing routine and
  important records
- **THEN** only the `important` records are printed

#### Scenario: Level is inclusive-and-above

- **WHEN** `gtmux events --severity notable` runs
- **THEN** both `notable` and `important` records are printed, and `routine` ones are omitted

#### Scenario: An invalid level is rejected

- **WHEN** `gtmux events --severity bogus` runs
- **THEN** the command reports the usage message rather than printing an unfiltered stream
