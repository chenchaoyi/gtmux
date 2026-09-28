## ADDED Requirements

### Requirement: A ready Codex composer clears an obsolete waiting marker

The radar SHALL clear a contradicted waiting marker and report `idle` when a
quiet Codex pane displays an input-ready composer instead of an approval menu.
A live approval menu or working pane SHALL keep its waiting signal.
For Codex, a strict live approval menu SHALL also report `waiting` when an
ownerless hook left no marker; the slow tick SHALL persist that screen-confirmed
wait and notify HQ once.

#### Scenario: A prior hook misattributed another session's approval

- **WHEN** an idle Codex pane displays its ready composer while a waiting marker exists
- **THEN** the marker is removed and `agents --json` reports `idle`

#### Scenario: Real approval remains on screen

- **WHEN** a Codex pane displays an active approval menu
- **THEN** its waiting marker is retained

#### Scenario: Ownerless hook leaves a live approval menu

- **WHEN** a Codex pane displays a live approval menu and its hook event had no pane identity
- **THEN** radar reports the pane as waiting and the slow tick records the wait for HQ
