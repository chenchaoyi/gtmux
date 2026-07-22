# mobile-chat-view — delta

## ADDED Requirements

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
