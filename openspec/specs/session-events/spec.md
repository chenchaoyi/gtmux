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

A live supervisor SHALL be woken for turn-ends that need attention — an `asking`
turn-end (a reply-text question that raised no menu) and any session's UNATTENDED
completion (a turn-end reaching idle while its pane is not the focused pane of an
attached tmux client), tracked dispatch or user-direct alike — through the wake
channel (see `hq-wake-protocol`), coalesced and per-pane rate-merged. Attended
completions and ordinary mid-conversation `report` turn-ends SHALL NOT each fire
a wake; they remain available by pull (`gtmux events`, `gtmux digest`) and are
counted into the summary tick. The wake SHALL be deduped per turn and gated on a
live HQ pane and the `hqNudge` setting.

#### Scenario: A reply-text question reaches HQ

- **WHEN** a turn ends with `class: "asking"` (no menu was raised) and an HQ pane is live
- **THEN** HQ receives one `asks` wake carrying the pane and the reply summary

#### Scenario: A user-direct session's unattended completion reaches HQ

- **WHEN** a session the user launched directly (not via dispatch) finishes work in
  an unfocused pane
- **THEN** HQ receives one `done` wake for it — tracked-dispatch status is not
  required

#### Scenario: Attended turns do not flood HQ

- **WHEN** an agent finishes a reply-turn in the pane the user currently has focused
- **THEN** no wake fires; the event is available in the stream and tallied for the
  next tick

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
LLM-free classifier from fields the record already holds (event/state/kind/class/origin)
and stamped at the SOURCE (the single append path), so it is persisted and queryable
without recompute.

Severity ranks URGENCY — how much someone is waiting on the supervisor — not relevance.
A `Waiting` event (the pane needs the user) and a `Stop` classified `asking` (a reply-text
question) SHALL be `important`; a `Stop` classified `report`, the session lifecycle events
(`SessionStart`/`SessionEnd`/`Resumed`/`PreCompact`), and a prompt submission carrying an
INSTRUCTION SHALL be `notable`; notifications, ordinary working ticks, and a submission
that carries no instruction SHALL be `routine`.

A prompt submission SHALL carry an additive `origin` field marking whether its payload is
a real instruction — typed prose or a slash command — as opposed to harness-injected
content or gtmux's own wake line echoed back. It SHALL be stamped from the SAME classifier
that decides whether the submission wakes HQ, so the wake and the tier can never disagree
about what a user act is. It SHALL NOT claim WHO authored the instruction: a task the
system dispatched carries one too, and no reliable signal distinguishes them (an
instruction reaching a session is a fleet change either way).

Both fields are additive to the stable event contract — a record without them (a legacy
line) SHALL read as `routine` with no instruction. Stamping SHALL NOT alter the existing
marker/notify state machines, and the write path SHALL remain fire-and-forget so a busy or
absent consumer never blocks the hook.

#### Scenario: A waiting event is important

- **WHEN** the hook appends a `Waiting` event (the pane blocked on the user)
- **THEN** the record's `severity` is `important`

#### Scenario: An asking turn-end is important, a report turn-end is notable

- **WHEN** a `Stop` event is classified `asking` versus `report`
- **THEN** the former's `severity` is `important` and the latter's is `notable`

#### Scenario: A submitted instruction is a fleet change, not chatter

- **WHEN** a `UserPromptSubmit` carrying typed prose or a slash command is appended
- **THEN** its `origin` is the instruction marker and its `severity` is `notable`, so a
  supervisor reading the fleet-change stream sees what the user told a session to do

#### Scenario: Injected content is not an act

- **WHEN** a `UserPromptSubmit` whose payload is harness-injected content (or gtmux's own
  wake line echoed back) is appended
- **THEN** it carries no `origin` and its `severity` is `routine`

#### Scenario: A legacy record without severity reads as routine

- **WHEN** a record written before this field is read back
- **THEN** it is treated as `routine` for severity purposes, without failing the read

### Requirement: Severity-filtered event read

`gtmux events` SHALL accept `--severity <level>` (`routine`|`notable`|`important`) to
restrict the stream to events at that level AND ABOVE (`routine` < `notable` < `important`),
applied to BOTH the bare recent-window form and `--follow`. An unrecognized level SHALL be
rejected with the usage message.

The command's own help SHALL NOT present any filtered read as "the attention stream".
There are three reads and they SHALL be described as what they are: the unfiltered delta
(`--since-seq <n>`) is the reconcile path; `--severity notable` is the fleet-change stream
(instructions, turn-ends, lifecycle); `--severity important` is the ESCALATION stream
(blocked, asking, crashed) — a subset, never the whole picture. Together with the
per-record `summary`, none of them requires reading a raw transcript to triage.

#### Scenario: Filter to escalation-worthy events

- **WHEN** `gtmux events --severity important` runs over a stream mixing routine and
  important records
- **THEN** only the `important` records are printed

#### Scenario: Level is inclusive-and-above

- **WHEN** `gtmux events --severity notable` runs
- **THEN** both `notable` and `important` records are printed, and `routine` ones are omitted

#### Scenario: The help does not oversell a filter

- **WHEN** `gtmux events --help` describes `--severity`
- **THEN** it names the escalation stream as a subset and points at the unfiltered
  `--since-seq` delta for reconciling, rather than calling any filter "the attention
  stream"

#### Scenario: An invalid level is rejected

- **WHEN** `gtmux events --severity bogus` runs
- **THEN** the command reports the usage message rather than printing an unfiltered stream

### Requirement: Events carry a monotonic sequence

Every event record SHALL carry an additive, strictly increasing `seq` field assigned at
the single append path from a persistent counter, so consumers have a total order and a
durable cursor position independent of file byte offsets (which rotation invalidates).
Concurrent appends SHALL each receive a distinct, increasing sequence (the counter is
serialized with an advisory file lock, cgo-free). The field is additive to the stable
event contract — a legacy record without it SHALL still read (treated as sequence-unknown,
ordered by timestamp) and MUST NOT fail the reader. Assigning the sequence SHALL keep the
append best-effort and fire-and-forget so a busy or absent consumer never blocks the hook.

#### Scenario: Each event gets an increasing sequence

- **WHEN** two events are appended, even concurrently by separate hook processes
- **THEN** each record carries a distinct `seq` and the later-assigned one is greater

#### Scenario: The sequence survives rotation

- **WHEN** the journal rotates and events keep being appended to the fresh file
- **THEN** the `seq` continues increasing across the rotation boundary (it is not reset)

#### Scenario: A legacy record without seq still reads

- **WHEN** a record written before this field is read back
- **THEN** it is read without error and ordered by timestamp

### Requirement: Resume-from-cursor subscription

The events reader SHALL support resuming from a consumer's cursor (a last-consumed `seq`):
a consumer that reconnects SHALL be able to replay EXACTLY the events with `seq` greater
than its cursor, across both the active and rotated generations, ordered by sequence, so a
crashed or restarted consumer loses no events and never re-emits a consumed one. The reader
SHALL expose enough to DETECT a gap (a missing sequence between the cursor and the next
available event) so a consumer can trigger reconciliation rather than proceed blind.

#### Scenario: Reconnect replays only the un-consumed tail

- **WHEN** a consumer with cursor N reconnects and the journal has advanced past N
- **THEN** it receives every event with `seq > N` once, in sequence order, spanning a
  rotation if one occurred

#### Scenario: A missing sequence is detectable

- **WHEN** the next available event's `seq` is not exactly one past what the consumer
  expected (a hole)
- **THEN** the reader surfaces that a gap exists so the consumer can reconcile

### Requirement: A failed turn is recorded as a crash, never a finish

The system SHALL record a `crash` event when an agent's turn dies on an agent/API
failure (Claude's `StopFailure` hook event), carrying the error head as DATA with
severity `important`, and SHALL NOT mark the pane's turn as a normal finish. A
live HQ SHALL be woken immediately with a `crash` wake line.

#### Scenario: An API-dead turn is not mistaken for done

- **WHEN** a session's turn aborts with an API error (StopFailure)
- **THEN** a `crash` event (severity important) is appended, no finished/idle
  marker is stamped as a normal completion, and HQ receives a `crash` wake

### Requirement: Sequence-filtered delta read

`gtmux events` SHALL support `--since-seq <n>`: a one-shot read of every retained
event with sequence strictly greater than `<n>`, in order, combinable with the
existing severity filter and JSON output (the existing `--since <duration>` time
window is unchanged). This is the pull-on-wake primitive: a supervisor woken with
a sequence range reads exactly the delta, on any agent capable of running a CLI
command.

#### Scenario: Pull the delta since the last wake

- **WHEN** HQ runs `gtmux events --since-seq 340 --json` after a wake covering seq
  341-352
- **THEN** it receives exactly the retained events with seq > 340, oldest first

### Requirement: The journal carries gtmux's own control and audit records

gtmux's own records SHALL ride the same journal and `Record` shape as agent
lifecycle events, namespaced apart from them by the `gtmux:` event prefix, and
SHALL be split into two kinds with different debt semantics:

- **Control records** (`gtmux:` outside the audit sub-namespace — maintenance
  triggers, degradations, reconciles) carry NEW information for the supervisor.
  They SHALL count toward the consumption debt, since the `unread` knock is the
  only channel a pane-less record has.
- **Audit records** (the `gtmux:audit:` sub-namespace) document an act whose
  actor already knows it — a delivered or dropped wake, a supervisor-driven send,
  a reap, a rotation, a knowledge-ledger operation. They are TRAIL, not debt:
  they SHALL NOT count toward the consumption debt, SHALL be omitted from the
  supervisor's default delta pull exactly as the tally omits them, and SHALL
  remain in the log — returned by `--all` and by any non-supervisor read.

Every audit record SHALL nest inside the control namespace, so the standing rule
that sensors exclude control records from the deltas they measure covers audit
records with no further case analysis. An audit record's severity SHALL be
`routine` — it never asks for attention.

Audit summaries SHALL be bounded and single-line: a payload longer than its
record's budget SHALL truncate at a rune boundary, and embedded newlines SHALL
collapse, so one record is always one journal line.

#### Scenario: An audit record is a control record

- **WHEN** a `gtmux:audit:wake-delivered` record is appended
- **THEN** it is classified as a control record (sensor deltas exclude it), renders
  through the control formatting of `gtmux events`, and carries routine severity

#### Scenario: An audit record never becomes debt

- **WHEN** the only records past the supervisor's watermark are `gtmux:audit:*`
- **THEN** no `unread` wake is raised for them, and they are still returned by
  `gtmux events --since-seq <n> --all` and by any non-supervisor read

#### Scenario: A maintenance trigger still becomes debt

- **WHEN** a `gtmux:distill` control record (not audit) lands past the watermark
- **THEN** it counts toward the consumption debt exactly as before

#### Scenario: An oversized audit payload stays one bounded line

- **WHEN** an audit record is constructed from a multi-line payload longer than its
  budget
- **THEN** the stored summary is single-line, truncated at a rune boundary, and the
  journal line parses as one record

### Requirement: A registered hook must mean something

gtmux asks each agent to run a command on a list of events, and separately holds a table
saying what those events mean. Nothing compared the two, so an event could be registered
with no mapping and be discarded on arrival without a warning anywhere — which is what
happened to the crash event: registered with Claude, given a branch in the state machine,
documented as a wake class and written into two specs, while every one that fired was
dropped. Zero were recorded in nineteen thousand events.

Every event the system registers with an agent SHALL resolve to a known meaning. An
event MISSING from an agent's own table SHALL fall through to the shared table rather
than resolving to nothing, so a forgotten entry degrades to the common meaning instead
of to silence. The pairing SHALL be enforced by a test, because the two lists live apart
and a reviewer reading either one cannot see the gap.

The reverse — a mapping with no registration — is NOT an error: a table may carry
meanings for events other agents send, or that an agent may gain later.

#### Scenario: An event registered with no mapping

- **WHEN** the system registers an event for which no table gives a meaning
- **THEN** the conformance test fails, naming the agent and the event

#### Scenario: An event missing from an agent's own table

- **WHEN** an agent sends an event its own table does not list but the shared table does
- **THEN** the shared meaning applies

### Requirement: A compaction is announced, not inferred

Agents announce that a context compaction has finished — the same turn then carries on.
The system SHALL treat that announcement as the turn continuing, and SHALL restore the
turn marker if it is absent, so a pane cannot come out of a compaction reporting idle
while it works.

#### Scenario: A turn that compacted mid-flight

- **WHEN** an agent reports a completed compaction for a pane
- **THEN** the pane reports `working`, whether or not it still had a turn marker

