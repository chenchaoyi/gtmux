# chat-transcript (delta)

## ADDED Requirements

### Requirement: A binding that has stopped moving is reported, not rendered as calm history

A pane's chat is served from the transcript its resume binding names. When that binding
goes stale — the agent moved the conversation to another session id and gtmux did not
learn of it — the served history is a complete, quiet, hours-old conversation that is
indistinguishable from a session that simply has nothing new to say. That silence is the
failure mode, so the system SHALL make it observable.

`gtmux doctor` SHALL report a pane whose bound transcript has not grown while the pane
itself has been active, naming the pane and how long the log has been silent. A pane that
is genuinely idle (no pane activity either) is NOT stale and SHALL NOT be reported.

#### Scenario: Active pane, silent log

- **WHEN** a pane's tmux activity is materially newer than the last content in the
  transcript its binding names
- **THEN** `doctor` reports that pane's binding as stale, with the pane id and the age

#### Scenario: A quiet session is not an error

- **WHEN** neither the pane nor its transcript has moved
- **THEN** nothing is reported

### Requirement: The chat view refreshes without needing a status change

The mobile chat SHALL keep its history current while the view is open, rather than
refetching only when the pane's status flips or the phone sends a prompt. A pane whose
status is itself stuck cannot flip, so a status-only trigger can never recover — which is
how a reader was left looking at five-hour-old history on a working pane.

`GET /api/transcript` SHALL support conditional requests so this polling is cheap: it
SHALL return a validator derived from the underlying log, and SHALL answer `304 Not
Modified` when the caller presents a matching validator.

#### Scenario: New turns appear without a status flip

- **WHEN** the chat view is open and the bound transcript grows while the pane's status
  does not change
- **THEN** the new turns appear

#### Scenario: Polling an unchanged transcript is nearly free

- **WHEN** the client polls with the validator it was last served and the log has not
  changed
- **THEN** the server answers `304` with no body
