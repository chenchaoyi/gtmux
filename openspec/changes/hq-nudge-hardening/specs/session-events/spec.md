# session-events Specification

## MODIFIED Requirements

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
