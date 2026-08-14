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

#### Scenario: A session with one window draws no window band

- **WHEN** a session holds exactly one window
- **THEN** the window level is not drawn as its own row — the session and the window
  coincide there, and a band per pane row would double every session's height — but the
  session header SHALL still name the window ids it holds, so a COLLAPSED session says what
  is inside it

#### Scenario: A plain shell pane is not labeled with the host name

- **WHEN** a plain shell pane's `pane_title` is empty or the machine host name
- **THEN** the row shows `%<pane_id>` plus the command/dir gloss, never the host-name title

### Requirement: The pane id is copyable as a command

Tapping or clicking a row's `%<pane_id>` SHALL copy `gtmux focus %<pane_id>` — the id made
runnable — and SHALL confirm in place, because a copy that shows nothing cannot be told from
a tap that missed. The gesture SHALL be scoped to the id: the row itself keeps its own
action (open the pane). Every surface SHALL copy the SAME string.

#### Scenario: Copying does not open the pane

- **WHEN** the user taps the `%N` on a row
- **THEN** the command is copied and the browser stays where it is; the pane opens only
  when the row itself is tapped

#### Scenario: The web surface copies without a secure context

- **WHEN** the web surface is reached over a plain `http://<lan-ip>:8765`, where
  `navigator.clipboard` does not exist
- **THEN** the copy still lands, via the pre-Clipboard-API path

#### Scenario: The ids are searchable

- **WHEN** the user types `%23` or `@17` (or the bare digits) into the browser's search
- **THEN** the matching pane / window rows are found — the id is a search key, not only a
  label
