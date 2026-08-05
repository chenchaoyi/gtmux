# pane-browser (delta)

## ADDED Requirements

### Requirement: The pane browser shows the tmux hierarchy with stable, sigil'd ids

Every pane-browser surface (mobile, menu-bar, web) SHALL render an explicit three-level
hierarchy — session → window → pane — where each tmux level carries its stable, sigil'd id
alongside a human gloss: the session by NAME, the window as `@<window_id> <window-name>`,
and the pane as `%<pane_id> <label>`. The pane label SHALL be gtmux's derived label (the
agent name + status for agent panes, else the command / directory), NOT the raw
`pane_title`. The `%<pane_id>` SHALL be visible on every row (not only an internal key), so
two panes are told apart by a stable id rather than only by the volatile label and the
mutable `window.pane` index.

#### Scenario: Two windows in one session are distinct

- **WHEN** a session has two windows whose indices differ but whose names collide (or are
  both empty/auto-named)
- **THEN** each appears under its own `@<window_id>` sub-group, told apart by the stable
  window id, not by a name or an index that repeats across sessions

#### Scenario: A plain shell pane is not labeled with the host name

- **WHEN** a plain shell pane's `pane_title` is empty or the machine host name
- **THEN** the row shows `%<pane_id>` plus the command/dir gloss, never the host-name title
