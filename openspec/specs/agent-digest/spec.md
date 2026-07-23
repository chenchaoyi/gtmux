# agent-digest Specification

## Purpose
TBD - created by archiving change supervisor-mvp. Update Purpose after archive.
## Requirements
### Requirement: Deterministic per-agent cognitive digest

The system SHALL assemble, on demand and without any LLM call, a digest for every
radar row (tmux and native) joining: identity (pane/loc/agent/source,
project/branch), state (waiting/working/idle/running + waiting kind + since +
errored/background markers), goal (the session's last user prompt), last (the
tail of the last assistant reply), when waiting — ask (the parsed prompt
options text), and the session's USAGE snapshot (`tok`, `ctx` 0–1, `rate`,
`usage_warn` — see `usage-watch`). Fields whose source is absent SHALL degrade
to empty without failing the row (zero-intrusion: agents need not cooperate).
The CLI SHALL remain cgo-free.

#### Scenario: Digest of a waiting agent

- **WHEN** an agent pane is waiting on a permission/plan/question
- **THEN** its digest row carries state=waiting with the kind, the goal from its
  transcript, and the ask text parsed from the live pane

#### Scenario: Sparse session degrades gracefully

- **WHEN** a session has no on-disk transcript (e.g. a just-started agent)
- **THEN** the digest row still renders from radar signals alone, with
  goal/last/ask and the usage fields empty

#### Scenario: Digest carries usage

- **WHEN** a session has usage data and a breached/projected layer
- **THEN** its digest row includes tok/ctx/rate and the `usage_warn` string

### Requirement: Digest CLI

The system SHALL provide `gtmux digest` printing a FORMATTED, COLUMN-ALIGNED
table (bilingual labels per `GTMUX_LANG`) — never a prose paragraph — and
`gtmux digest --json` emitting a machine-readable array; together these are
the supervisor's primary read surface. The text form SHALL render: a one-line
summary of counts by state, then one section per state (needs-you first, then
working, then completed, then errored — the last only when non-empty) with
one aligned row per agent (status glyph · name · goal/last/ask, truncated to
the terminal width · a right-side badge · a right-aligned relative time).

#### Scenario: Fleet at a glance

- **WHEN** the user (or the supervisor agent) runs `gtmux digest --json`
- **THEN** every radar row appears with the digest fields, ordered like the
  radar (needs-you first)

#### Scenario: Scannable table, not prose

- **WHEN** the user runs `gtmux digest` (no `--json`) with live agents
- **THEN** the output opens with a one-line count-by-state summary, followed
  by a section per non-empty state, each row column-aligned and truncated to
  fit the terminal width — no free-form paragraphs

### Requirement: Digest over the API

The system SHALL expose `GET /api/digest` (bearer-gated like every `/api/*`)
returning the same JSON array, additively — existing API contracts unchanged.

#### Scenario: Remote digest read

- **WHEN** an authenticated client GETs `/api/digest`
- **THEN** it receives the digest array; without a token the request is rejected
  like any other `/api/*`

### Requirement: Digest rows surface a dispatched task's goal and status

A digest row whose pane has a dispatch-ledger entry (from `gtmux spawn`) SHALL
carry the dispatched task's goal and its lifecycle status (delivered → working →
waiting → done) as additive fields. Rows for panes with no ledger entry SHALL be
unchanged (the fields absent). The status SHALL be derived from the pane's live
radar state, consistent with `gtmux tasks`.

#### Scenario: A dispatched pane shows its task

- **WHEN** a pane was dispatched via `gtmux spawn` and `gtmux digest --json` runs
- **THEN** its row additionally carries the dispatched goal and lifecycle status

#### Scenario: Untracked panes are unchanged

- **WHEN** a pane was not dispatched via `gtmux spawn`
- **THEN** its digest row carries no dispatch fields (fully additive)

### Requirement: Digest marks an input-locked pane

The `gtmux digest --json` / `GET /api/digest` contract SHALL carry an additive,
optional `in_mode` boolean, mirroring the radar: true when the pane is in tmux
copy-mode / view-mode (input-locked), absent (omitempty) otherwise. This lets the
supervisor see at a glance which pane is currently swallowing input, distinct from its
working/waiting/idle status.

#### Scenario: A digest row reflects the input lock

- **WHEN** a pane is in copy/view-mode and a client reads `gtmux digest --json` or
  `GET /api/digest`
- **THEN** that pane's digest row carries `in_mode:true`
- **AND** rows for panes not in a mode omit `in_mode`

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
