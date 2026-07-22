# Mobile Chat View Specification

## Purpose

Render an agent's parsed transcript (see `chat-transcript`, fed by
`/api/transcript`) as a readable conversation on the phone — so a glance at "对话/
chat" tells you what the agent did and said without parsing its raw terminal. It is
the mobile client renderer of the transcript capability; the browser mirror renders
the same data (see `browser-mirror`).

## Requirements

### Requirement: Conversation of turns with markdown

The system SHALL render each turn as a user-prompt bubble followed by the agent's
reply rendered as markdown, kept fresh as the transcript grows, with an empty/
loading state when there is no history yet.

#### Scenario: Render a turn

- **WHEN** a transcript turn is shown
- **THEN** the user's prompt appears as a bubble and the agent's reply renders as
  markdown

### Requirement: Multi-segment replies as separate speech bubbles

The system SHALL render a reply's `segments` as separate speech bubbles in
chronological order, with the agent avatar on the first text bubble only and the
tool steps that ran between texts shown as a collapsible group (tapping expands the
steps) between the bubbles — so interleaved process reads clearly and segment
boundaries are obvious.

#### Scenario: Steps between bubbles

- **WHEN** a turn has text → tools → more text
- **THEN** each text shows as its own bubble and a collapsible "N steps" group sits
  between them, expandable on tap

### Requirement: Per-turn time label with date

The system SHALL show a per-turn time label derived from the prompt timestamp,
including the date for clarity (relative "today/yesterday" wording, calendar date
otherwise, and the year when not the current year), localized en/zh, and SHALL
de-duplicate the label when adjacent turns share it.

#### Scenario: Dated label

- **WHEN** a turn carries a prompt timestamp
- **THEN** its label shows the date and time (e.g. "today HH:MM" / a calendar date),
  in the device language

### Requirement: Long-press to select and Copy

The system SHALL let the user long-press to select and Copy text in the chat view
(prompt bubbles and rendered reply blocks), matching the copy affordance of the
terminal view.

#### Scenario: Copy a reply

- **WHEN** the user long-presses a reply bubble
- **THEN** text selection + the Copy callout are offered

### Requirement: Chat renders a window of recent turns, not the whole history

The chat view SHALL render only a window of the most recent turns rather than the entire
transcript, and SHALL offer an explicit control to extend that window further back. This
is required because the view mounts every turn eagerly with replies expanded: a real
long-running session has produced thousands of reply bubbles and tool-step rows, whose
combined view count exhausts device memory and terminates the app when the user switches
to Chat. The window SHALL keep the NEWEST turns, since the tail is what the reader opened
Chat to see. When turns are hidden — whether by this window or dropped by the server —
the view SHALL say so and say how many, rather than presenting a partial conversation as
the whole one.

#### Scenario: Opening a long conversation

- **WHEN** the user switches to Chat on a session with far more turns than the window
- **THEN** the most recent turns are rendered and the app remains running

#### Scenario: Reaching further back

- **WHEN** the user asks for earlier turns
- **THEN** the window extends toward the start of the available history

#### Scenario: Truncation is disclosed

- **WHEN** any turns are not being shown, whether windowed away or dropped before
  reaching the client
- **THEN** the view states that earlier history is not shown and how many turns that is
