# mobile-app (delta)

## MODIFIED Requirements

### Requirement: HQ's chat shows what is still running

The HQ task row SHALL be rendered by the HQ page used by both the compact and regular
shells. The demonstration's tasks SHALL refer to existing demonstration panes and
follow those panes' status changes.
The split shell SHALL prefer an ordinary pane at startup, and SHALL use the HQ page
when HQ is the only session.

#### Scenario: A task waiting in the iPad HQ page

- **WHEN** a dispatched task is waiting and HQ is open in the iPad split shell
- **THEN** the row above the HQ composer opens the task list and its task opens the matching pane in the main area

### Requirement: Opening the row lists the work and leads to it

The task list SHALL be centred and limited to a reading width on a regular iPad
canvas. The share-link delivery sheet SHALL follow the same rule. Both SHALL
remain bottom sheets on compact canvases.

#### Scenario: Opening a task list on a full iPad canvas

- **WHEN** the reader taps the HQ task row
- **THEN** the list does not stretch to the full canvas width
