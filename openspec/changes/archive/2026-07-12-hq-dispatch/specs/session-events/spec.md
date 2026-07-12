# session-events Specification

## ADDED Requirements

### Requirement: Turn-end events carry a reply summary and class

Every turn-end (`Stop`) event SHALL carry two additive fields: `summary` — the tail
of the assistant's last reply (the same extraction the digest uses for `last`) —
and `class`, a deterministic classification of the turn-end. `class` SHALL be
`asking` when the reply's last non-empty sentence is a question directed at the
user (e.g. ends with `?`/`？` after stripping code/quote lines), and `report`
otherwise. Both fields are additive to the stable event contract and absent when
no reply text is available (e.g. a non-cooperative agent). The classification SHALL
be deterministic and require no LLM tokens.

#### Scenario: A finishing turn records its reply and class

- **WHEN** an agent finishes a turn (`Stop`) whose reply ends on a question to the user
- **THEN** the emitted event carries the reply `summary` and `class: "asking"`

#### Scenario: A plain finishing turn is a report

- **WHEN** an agent finishes a turn whose reply is not a question to the user
- **THEN** the emitted event carries the reply `summary` and `class: "report"`

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
