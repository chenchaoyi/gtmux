# agent-radar (delta)

## ADDED Requirements

### Requirement: The radar captures the stable window (and session) tmux ids

The radar's pane queries (`agents --json`, `panes --json`) SHALL additively capture the
tmux `window_id` (`@N`) — and MAY capture the session `session_id` (`$N`) — so a window
can be grouped and addressed by an id that survives renaming and reordering, not only by
its mutable index. These fields SHALL be optional and additive to the JSON contract.

The new tmux-identity fields SHALL be named so they do NOT collide with the existing
`session_id` JSON field, which is the coding-agent ADOPT key (populated for `source:
"native"` rows) and is NOT tmux's `$N`. A distinct name such as `win_id` /
`tmux_session_id` SHALL be used.

#### Scenario: A renamed session keeps its panes' window identity

- **WHEN** a session is renamed or its windows are reordered
- **THEN** each pane's captured `@<window_id>` is unchanged, so a consumer grouping by
  window id sees the same windows

#### Scenario: The adopt key is not overloaded

- **WHEN** a consumer reads the tmux session id and the coding-agent adopt id
- **THEN** they are distinct JSON fields; the tmux id does not overwrite or alias the
  existing `session_id` adopt key
