# agent-digest (delta)

## ADDED Requirements

### Requirement: Digest rows annotate their perception tier

The `gtmux digest --json` / `GET /api/digest` contract SHALL carry an additive,
optional `sense` field per row annotating how that row is perceived: `driver`
when both the state truth (hook state machine) and the content (transcript)
come from structured, agent-produced sources; `partial` when only one of the
two does; `screen` when the row rests on capture/process heuristics alone. The
annotation SHALL be derived from facts the system already holds (the waiting
marker's source, transcript reachability) with no new collection, SHALL be
`omitempty` (absent on legacy readers' expectations), and SHALL NOT alter any
existing field or ordering. Consumers MAY weight their trust in a row by its
tier; the field is informational and changes no behavior by itself.

#### Scenario: A hook-and-transcript session reads as driver-grade

- **WHEN** a digest row's state comes from the hook state machine and its
  goal/last come from the session transcript
- **THEN** the row carries `sense: "driver"`

#### Scenario: A hook-less agent reads as screen-grade

- **WHEN** a digest row belongs to an agent with no gtmux hook and no transcript
  mapping, so its state is classified from the screen/process signals
- **THEN** the row carries `sense: "screen"`, and all other fields are exactly as
  before this change

#### Scenario: Legacy consumers are unaffected

- **WHEN** an existing consumer parses `digest --json` ignoring unknown fields
- **THEN** it observes no change other than the presence of the optional `sense`
  key
