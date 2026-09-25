# mobile-app (delta)

## ADDED Requirements

### Requirement: HQ's chat shows what is still running

In HQ's chat, above the composer, the app SHALL show one row when anything dispatched is
still running. It SHALL say how many are running, and SHALL say separately when one or more
of them is waiting on the user, because that is the state that needs a person. When nothing
is running the row SHALL NOT be rendered.

The row SHALL read as a control rather than as a status line: its own surface and a
chevron, in the same visual language as the approval card that already sits there. Colour
SHALL continue to carry state only, never tappability.

The row SHALL NOT appear in a worker pane's chat, because that pane is itself one of the
running things.

#### Scenario: Something is waiting on the user

- **WHEN** one dispatched task is waiting for input and two others are working
- **THEN** the row says so in two parts, with the waiting part in the waiting colour

#### Scenario: Nothing is running

- **WHEN** no dispatched task is running
- **THEN** no row is rendered and the composer sits where it always does

### Requirement: Opening the row lists the work and leads to it

Tapping the row SHALL open a sheet listing the tasks in two groups, running and finished,
with the finished group carrying its count and both collapsible. Each row SHALL carry the
goal, the agent, the pane, and how long the task has been in that state.

Tapping a row SHALL navigate to that task's pane. A task whose pane is gone SHALL be shown
dimmed and SHALL NOT offer navigation.

#### Scenario: Going to the work

- **WHEN** the user taps a running task in the sheet
- **THEN** the app opens that task's pane

#### Scenario: A task whose pane was closed

- **WHEN** a task's pane no longer exists
- **THEN** its row is dimmed, carries no chevron, and does not navigate
