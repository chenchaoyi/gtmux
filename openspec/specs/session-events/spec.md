# session-events Specification

## Purpose
TBD - created by archiving change session-events. Update Purpose after archive.
## Requirements
### Requirement: Append-only session event log

The system SHALL append one JSON record per agent lifecycle event — for every
session, tmux or native — to a bounded log at `~/.local/share/gtmux/events.jsonl`,
fed by the SAME hook that writes the state markers and the notify queue (additive;
those are unchanged). Each record SHALL carry at least a timestamp, the event, the
derived state, and the session's identity (pane/loc/session/agent) plus the
waiting kind when applicable. The log SHALL ROTATE at a size cap — the active file
renamed to `events.1.jsonl` (overwriting any prior) and a fresh one started,
keeping one rotated generation — so total on-disk size is bounded (active + 1
rotated; default 20 MB cap → ≈ 40 MB ceiling) and it can never single-point-explode.

#### Scenario: Every event is logged

- **WHEN** the hook fires for any session (start/stop/waiting/…)
- **THEN** a JSON line for it is appended to events.jsonl, with ts/event/state/
  identity, without altering the existing markers or notify queue

#### Scenario: The log rotates and stays bounded

- **WHEN** the active log exceeds its size cap
- **THEN** it is rotated to a numbered generation and a fresh file starts; old
  generations beyond the retained count are removed, so total size stays bounded

#### Scenario: Follow survives rotation

- **WHEN** `gtmux events --follow` is running and the log rotates
- **THEN** the follower re-opens the new file and keeps emitting new events (it
  does not silently stop)

### Requirement: Events reader and subscription

The system SHALL provide `gtmux events [--follow] [--json] [--since <dur>]` to
read the recent stream or `--follow` it live — the terminal-native subscription
to all sessions' execution, usable by gtmux HQ and any script.

#### Scenario: Tail the live stream

- **WHEN** a consumer runs `gtmux events --follow`
- **THEN** it receives existing recent events and then each new event as it is
  appended, until interrupted

#### Scenario: Recent window

- **WHEN** `gtmux events --since 10m` is run
- **THEN** only events from the last 10 minutes are printed

### Requirement: Turn-end events carry a reply summary and class

Every turn-end (`Stop`) event SHALL carry two additive fields: `summary` — the tail
of the assistant's last reply (the same extraction the digest uses for `last`) —
and `class`, a deterministic classification of the turn-end. `class` SHALL be
`asking` when a question directed at the user appears in the reply's TRAILING BLOCK —
any of the last several prose lines ends with `?`/`？` (after stripping code fences,
block quotes, and headings) — NOT only the single final line, so a question followed
by a status sign-off is still classified `asking`; otherwise `report`. The
classification SHALL apply to every session regardless of how it was created
(spawn-dispatched or manually started/resumed) — it is keyed on the reply text, not
the dispatch ledger. Both fields are additive to the stable event contract and
absent when no reply text is available (e.g. a non-cooperative agent). The
classification SHALL be deterministic and require no LLM tokens.

#### Scenario: A finishing turn records its reply and class

- **WHEN** an agent finishes a turn (`Stop`) whose reply ends on a question to the user
- **THEN** the emitted event carries the reply `summary` and `class: "asking"`

#### Scenario: A question followed by a sign-off is still asking

- **WHEN** a reply poses a question to the user and then ends with a short status /
  sign-off line that is not itself a question
- **THEN** the turn-end is classified `class: "asking"`, not `report`

#### Scenario: A plain finishing turn is a report

- **WHEN** an agent finishes a turn whose trailing block asks the user nothing
- **THEN** the emitted event carries the reply `summary` and `class: "report"`

#### Scenario: Classification is not gated on dispatch source

- **WHEN** a manually-started (non-ledger) session finishes a turn that asks the user
  a question and an HQ pane is live
- **THEN** the event is classified `asking` and HQ receives the `asks` nudge, the
  same as for a spawn-dispatched session

### Requirement: Attention-worthy turn-ends push an HQ nudge without flooding

A live supervisor SHALL be pushed a nudge for turn-ends that need attention — an
`asking` turn-end (a reply-text question that raised no menu) and the completion of
a tracked dispatch — while ordinary `report` turn-ends SHALL NOT each fire a nudge
(they remain available by subscribing to the stream, e.g. `gtmux events --follow`).
The push SHALL be deduped per turn (one nudge per turn-end) and gated on a live HQ
pane and the `hqNudge` setting.

#### Scenario: A reply-text question reaches HQ

- **WHEN** a turn ends with `class: "asking"` (no menu was raised) and an HQ pane is live
- **THEN** HQ receives one `asks` nudge carrying the pane and the reply summary

#### Scenario: Ordinary turns do not flood HQ

- **WHEN** an agent finishes a `report` turn that needs no decision
- **THEN** no nudge is pushed; the event is available in the session-events stream

### Requirement: Prompt submissions and compaction are recorded for verify

A `UserPromptSubmit` event SHALL additionally record the submitted prompt's
normalized content head (in the additive `summary` field), so a dispatcher can
match a submission deterministically from the stream rather than by screen-reading.
A `PreCompact` event SHALL be emitted as a state-neutral lifecycle event (it does
not change the pane's working/waiting/idle marker), so a `/compact` can be confirmed
from the stream. Both are additive and MUST NOT alter the existing marker state
machine.

#### Scenario: A prompt submission is matchable from the stream

- **WHEN** an agent fires `UserPromptSubmit` for a delivered prompt
- **THEN** the emitted event carries the prompt's normalized head, and a dispatcher
  polling the stream can confirm the submission without reading the screen

#### Scenario: Compaction is observable

- **WHEN** a session begins compaction (`PreCompact`)
- **THEN** a state-neutral `PreCompact` event is emitted and the pane's status marker
  is unchanged

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

