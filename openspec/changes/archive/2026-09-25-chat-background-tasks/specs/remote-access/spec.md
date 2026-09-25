# remote-access (delta)

## ADDED Requirements

### Requirement: serve reports what was dispatched and whether it is still running

`GET /api/tasks` SHALL return the dispatch ledger's entries joined with each task's live
pane status: `waiting`, `working`, `idle`, or `gone` when the pane no longer exists. Each
entry SHALL carry its id, goal, agent, pane and the time it was dispatched.

The endpoint SHALL be owner-only: a guest token SHALL be refused.

#### Scenario: A dispatched task that is now waiting on the user

- **WHEN** a task was dispatched to a pane that is now waiting for input
- **THEN** `GET /api/tasks` reports that task with status `waiting`

#### Scenario: The pane is gone

- **WHEN** a task's pane no longer exists
- **THEN** the task is still reported, with status `gone`

#### Scenario: A guest asks

- **WHEN** a guest-scoped token calls `GET /api/tasks`
- **THEN** it is refused, because a share link is scoped to panes and the ledger is not
