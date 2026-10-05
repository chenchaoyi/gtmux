# agent-digest Specification

## Purpose
Provide a deterministic fleet digest for the CLI and owner API, joining radar
state with available conversation, usage, and dispatch records without an LLM call.
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

- **WHEN** an agent pane is waiting on a permission/plan/question with a clean,
  replyable numbered menu
- **THEN** its digest row carries state=waiting with the kind, the goal from its
  transcript when available, and the ask text parsed from the live pane

#### Scenario: A waiting pane has no replyable numbered menu

- **WHEN** a waiting pane has no parsed options or its parsed menu is not replyable
- **THEN** its digest row SHALL omit `ask` rather than invent numbered choices

#### Scenario: Sparse session degrades gracefully

- **WHEN** a session has no on-disk transcript (e.g. a just-started agent)
- **THEN** the digest row still renders, with `goal`, `last` and usage fields empty
- **AND** `ask` is populated independently when the live pane has replyable options

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

The system SHALL expose bearer-authenticated `GET /api/digest` as an owner-only
whole-fleet view returning the same JSON array as the CLI. A scoped guest SHALL
receive 403 even when some panes are shared with it. Missing or invalid bearer
credentials SHALL receive 401.

#### Scenario: Remote digest read

- **WHEN** a client with valid owner credentials GETs `/api/digest`
- **THEN** it receives the digest array

#### Scenario: A guest cannot read the whole-fleet digest

- **WHEN** a client with valid guest credentials GETs `/api/digest`
- **THEN** it receives 403 rather than a whole-fleet or filtered digest

### Requirement: Digest rows surface a dispatched task's goal and status

A digest row whose pane has a dispatch-ledger entry (from `gtmux spawn`) SHALL
carry the dispatched task's goal and `task_status` as additive fields. A dispatch
that the ledger records as neither delivered nor queued by the agent SHALL read
`undelivered`, regardless of its pane's current state. Otherwise the status SHALL
be derived from the live radar state: waiting → `waiting`, idle → `done`, and
other states → `working`, consistent with `gtmux tasks`. These are observations,
not a required sequence. Rows with no matching ledger entry SHALL omit both fields;
the digest SHALL NOT add rows for panes absent from the radar.

#### Scenario: A dispatched pane shows its task

- **WHEN** a pane was dispatched via `gtmux spawn` and `gtmux digest --json` runs
- **THEN** its row additionally carries the dispatched goal and lifecycle status

#### Scenario: An idle pane does not turn an undelivered task into done

- **WHEN** the ledger records a failed, unqueued delivery and its pane is idle
- **THEN** the digest row carries `task_status: "undelivered"`, not `done`

#### Scenario: Untracked panes are unchanged

- **WHEN** a pane was not dispatched via `gtmux spawn`
- **THEN** its digest row carries no dispatch fields (fully additive)

#### Scenario: A reused pane ID is not the dispatched pane

- **WHEN** a ledger entry was recorded before the running tmux server started, and a new
  pane now has the same ID (tmux numbers panes from `%0` again on each server start)
- **THEN** the new pane's row carries no dispatch fields, done notices do not name that
  entry's goal, and `gtmux spawn` does not resume that entry into the new pane

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
optional `sense` field per row reporting the session/content lookup results.
It SHALL be `driver` when a session record resolves and its registered, enabled
content reader returns without an error; `partial` when the record resolves but
the reader is absent, disabled, or returns an error; and `screen` when no session
record resolves. The transcript lookup requires that record. The content reader
returns no error for a missing log as well as for an empty transcript, so `driver`
does not prove that a transcript file or conversation content exists.

The annotation SHALL use these existing lookup results without new collection,
SHALL be `omitempty` when unset, and SHALL NOT alter existing fields or ordering.
It describes lookup results, not proof that every state classification came from
a hook: screen/process inference remains part of radar classification.
The field is informational and changes no behavior by itself.

#### Scenario: A hook-and-transcript session reads as driver-grade

- **WHEN** a digest row has a resolved session record and its registered, enabled
  content reader returns without an error
- **THEN** the row carries `sense: "driver"`

#### Scenario: No log yet can still produce the driver tier

- **WHEN** a session record resolves but no log file exists, and its registered,
  enabled content reader returns no turns and no error
- **THEN** the row carries `sense: "driver"` and omits `goal`, `last` and usage fields
- **AND** any replyable options parsed from the live pane remain independent

#### Scenario: Disabling content leaves a partial digest

- **WHEN** a digest row has a resolved session record but its driver's content
  capability is disabled
- **THEN** the row carries `sense: "partial"` and omits `goal` and `last`
- **AND** radar and ask are gathered as before; usage is still read from the session
  log independently of the content capability switch, when that log exists

#### Scenario: A hook-less agent reads as screen-grade

- **WHEN** a digest row belongs to an agent with no gtmux hook and no transcript
  mapping, so its state is classified from the screen/process signals
- **THEN** the row carries `sense: "screen"`, and all other fields are exactly as
  before this change

#### Scenario: Legacy consumers are unaffected

- **WHEN** an existing consumer parses `digest --json` ignoring unknown fields
- **THEN** it observes no change other than the presence of the optional `sense`
  key
