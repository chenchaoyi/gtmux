# session-events — delta

## MODIFIED Requirements

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

## ADDED Requirements

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
