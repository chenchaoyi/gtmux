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
