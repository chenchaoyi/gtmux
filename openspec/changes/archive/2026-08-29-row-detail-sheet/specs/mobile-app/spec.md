# mobile-app (delta)

## ADDED Requirements

### Requirement: Long-pressing a session says what the row cannot

A radar row is clamped for density, so the long-press exists to reveal what the clamp
hides. It SHALL show the full task, the full error text where the session ended on one,
and the background-work note where work is still in flight — and it SHALL identify the
pane it belongs to, leading with the pane id, because a truncated name cannot tell two
panes running one project apart.

It SHALL NOT merely restate the row. Repeating the agent name and the task — both already
visible on the row that was pressed — spends a deliberate gesture on nothing.

It SHALL offer the step that is otherwise two screens away: jumping the Mac's terminal to
that pane, and copying the command that does the same thing from a shell. It SHALL NOT
offer a destructive action; a long-press is a browsing gesture.

#### Scenario: A task or error too long for the row

- **WHEN** a row's task or error text is clamped
- **THEN** the long-press shows it in full

#### Scenario: Identifying which pane this is

- **WHEN** the sheet opens for a pane in tmux
- **THEN** it leads with the pane id, with the session/window location beside it

### Requirement: Each kind of row tells its own truth

The radar carries agent sessions, sessions sensed outside tmux, and plain panes a user
promoted onto the list. They are different things, and the sheet SHALL NOT present them
identically.

A session sensed outside tmux SHALL be shown as such, and the actions that require a pane
SHALL be offered as unavailable WITH THE REASON rather than silently absent or,
worse, offered and broken. A watched plain pane SHALL NOT be given an agent status it
does not have.

#### Scenario: A session that is not in tmux

- **WHEN** the sheet opens for a row sensed outside tmux
- **THEN** it says so, and the jump is unavailable with the reason given

#### Scenario: A watched plain pane

- **WHEN** the sheet opens for a promoted plain pane
- **THEN** it shows the pane and its command, and no agent status
