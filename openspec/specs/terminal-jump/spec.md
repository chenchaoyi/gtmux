# Terminal Jump Specification

## Purpose

Bring the terminal tab + tmux pane for a given session to the front in one
action, so a notification or a list row lands you exactly where an agent needs
you. This is the "remote control" side; it drives the host terminal and is
distinct from the terminal-agnostic radar.
## Requirements
### Requirement: Jump to a session or pane

The system SHALL, given a session name or a tmux pane id (`%N`), select that
window+pane in tmux and bring the host terminal's tab showing that session to
the front. It SHALL inject no input and run no command (read-only jump).

#### Scenario: Focus by pane id

- **WHEN** `gtmux focus %12` is run
- **THEN** tmux selects window+pane for `%12` and the host terminal tab showing
  that session is activated

#### Scenario: Focus the last finished agent

- **WHEN** `gtmux focus --last` is run
- **THEN** the most-recently-finished agent's pane is focused

### Requirement: Host terminal abstraction

The system SHALL drive the host terminal through a `Terminal` driver interface
(`FocusTab`/`IsViewing`/`OpenWindow`/`SpawnTabs`) and SHALL auto-detect the host
terminal, with a `GTMUX_TERMINAL` override.

#### Scenario: Detect the host

- **WHEN** resolving the active terminal
- **THEN** the system uses `GTMUX_TERMINAL` if set, else `$TERM_PROGRAM`, else
  the tmux client's process ancestry, else falls back to Ghostty

#### Scenario: Supported drivers

- **WHEN** the host terminal is Ghostty (1.3+) or iTerm2
- **THEN** focus/restore/new work via that driver's AppleScript

### Requirement: Warp is driven best-effort (no per-tab scripting)

Warp ships no AppleScript scripting dictionary, so the tab-title iteration the
other drivers use is impossible. The system SHALL still register a Warp driver
(detected via `TERM_PROGRAM=WarpTerminal` or a `Warp.app` process ancestry) so a
Warp-hosted session is never driven through another terminal's driver. The
driver SHALL focus by opening `warp://session/<uuid>` when the session's tmux
environment carries a recorded `WARP_TERMINAL_SESSION_UUID`, and SHALL fall back
to activating the Warp app otherwise. `restore`/`new` SHALL work via Warp launch
configurations (`~/.warp/launch_configurations` + `warp://launch/<name>`), whose
attach step records each tab's Warp session uuid into the tmux session
environment — making later focus precise for gtmux-opened tabs. `IsViewing`
SHALL match the frontmost process by Warp's real process name (`stable`) and the
window title, answering false when the title cannot confirm the session (never
wrongly suppressing a notification). `gtmux doctor` SHALL state the best-effort
limits rather than claiming full support.

#### Scenario: Focus a Warp-hosted session

- **WHEN** `gtmux focus` targets a session whose tmux client runs inside Warp
- **THEN** the Warp driver is used (never a fallback driver aimed at another
  app): the exact tab via `warp://session/<uuid>` when a uuid is recorded, else
  the Warp app is activated

#### Scenario: Restore into Warp tabs

- **WHEN** `gtmux restore` runs with Warp as the host terminal
- **THEN** a launch configuration opens one tab per session, each attaching its
  tmux session and recording that tab's Warp session uuid for later focus

### Requirement: Tab matching by title

The system SHALL match a session's tab by the tmux title `#S — #W`, and therefore
requires `set-titles on` with `set-titles-string '#S — #W'`.

#### Scenario: Title-based match

- **WHEN** the terminal exposes a tab/session whose title is `<session> — <window>`
- **THEN** focus matches it by that prefix (absorbing terminal-specific suffixes
  such as iTerm2's ` (tmux)`)

### Requirement: Restore tabs after the terminal quits

The system SHALL, via `gtmux restore`, open one terminal tab per tmux session and
attach them, reusing the current tab when invoked inside one.

#### Scenario: Reattach after quitting the terminal

- **WHEN** the terminal was quit (tmux sessions still alive) and `gtmux restore`
  is run
- **THEN** one tab per session is opened and attached

### Requirement: Jump and type target any pane regardless of agent

`gtmux focus <pane-id>` and `gtmux send <pane-id>` SHALL operate on any live tmux
pane, whether or not it runs a coding agent — the jump/type primitives are pane-level
and do not require the target to be on the agent radar. When the target pane exists
but is not (or is no longer) running an agent, `gtmux focus` SHALL still jump to it
(its content is often exactly what the user wants to see) and SHALL make clear that
the pane is a plain shell rather than silently landing the user on it as if it were
still an agent. When the target pane no longer exists at all, `gtmux focus` SHALL
report that instead of jumping.

#### Scenario: Focus a pane whose agent has exited

- **WHEN** `gtmux focus %N` targets a pane that exists but whose coding agent has since
  exited (it is now a plain shell)
- **THEN** focus jumps to the pane and states that the agent there has exited and it is
  now a plain shell

#### Scenario: Focus a plain pane that never ran an agent

- **WHEN** `gtmux focus %N` targets a live plain pane that is not on the agent radar
- **THEN** focus jumps to it normally, treating a non-agent pane as a first-class jump
  target

### Requirement: The terminal tab can carry every pane id in its window

`doctor` SHALL report whether the terminal tab names the panes behind it, and `doctor --fix`
SHALL OFFER — never silently apply — the tmux configuration that makes it so: an
`automatic-rename-format` that emits every `#{pane_id}` in the window (`#{P:…}`), plus a
`pane-exited` hook that forces the name to be recomputed.

The ids SHALL reach the tab through the WINDOW NAME, not through `set-titles-string`.
Because `set-titles-string` is `#S — #W`, the tab inherits them with no change to the title
format — and therefore the terminal drivers' title MATCHERS
(`FocusTab`/`IsViewing`/`TabOrder`) SHALL NOT need to change. The ids SHALL NOT be placed
ahead of the session name in the title, which is what would break the matchers.

The suggested format SHALL list every pane in the window, independent of which pane is
active: tmux re-evaluates the format on its own schedule, so an ACTIVE-pane id lags the
real active pane and is a stale pointer dressed as a live one (measured).

The refresh hook SHALL be treated as part of the configuration, not decoration: adding a
pane re-evaluates the window name immediately, but REMOVING one does not (measured), so
without the hook the tab keeps advertising a pane that no longer exists.

gtmux SHALL NOT rename the user's windows to achieve this. `rename-window` turns
`automatic-rename` OFF for that window, permanently overwriting whatever format the user
configured.

#### Scenario: A tab maps to the pane HQ named

- **WHEN** HQ refers to pane `%11` and the user looks at their terminal tabs
- **THEN** the tab of the window holding `%11` shows `%11` in its title, whether or not
  `%11` is the pane that window currently has selected

#### Scenario: Closing a pane does not leave a phantom id in the tab

- **WHEN** a pane is closed from a window whose name lists it
- **THEN** the `pane-exited` hook re-evaluates the name and the closed pane's id is gone

#### Scenario: Jumping still works with ids in the title

- **WHEN** the window name carries pane ids and `gtmux focus <session>` runs
- **THEN** the tab is still matched by its session prefix, because the ids land after the
  `#S — ` separator

#### Scenario: An existing custom format is not replaced

- **WHEN** the user already has an `automatic-rename-format` of their own and accepts
  `doctor --fix`
- **THEN** the pane ids are APPENDED to their format rather than replacing it; a format
  that is merely tmux's default may be replaced

### Requirement: The window-name source is reported, not silently changed

`doctor` SHALL report what a window name is made of, and SHALL flag tmux's DEFAULT
(`#{pane_current_command}`) as worth improving: for a Claude 2.x pane that renders the
agent's VERSION string, so a default user's tab reads `<session> — 2.1.229`. A format the
user has customized SHALL be reported as-is and left alone.

#### Scenario: A default install is told why its tabs read like version numbers

- **WHEN** `automatic-rename-format` is unset or tmux's default
- **THEN** `doctor` shows the row as improvable and names the directory-based alternative

#### Scenario: A customized format is respected

- **WHEN** the user has set their own `automatic-rename-format`
- **THEN** `doctor` reports it and marks the row OK

### Requirement: A session with nothing showing it is opened, not silently missed

When a jump targets a session that has NO attached terminal client, the system SHALL open a
terminal tab attached to that session rather than searching for a tab that cannot exist. The
condition SHALL be measured from the client count, not from how the session was started: a
session spawned `--headless` that someone later attached is an ordinary jump.

Surfaces SHALL mark a row whose session has no attached client, so the user knows before
clicking that a window will be opened for it.

#### Scenario: Jumping to a headless dispatch

- **WHEN** a session was spawned with `--headless` (no tab was opened) and the user clicks
  its row
- **THEN** a terminal tab attached to that session is opened, instead of the click doing
  nothing at all

#### Scenario: A headless session that was attached later

- **WHEN** a session carries the headless marker in its window name but a client is attached
- **THEN** it is jumped to like any other session, and it is NOT marked as having no window

#### Scenario: The row says so first

- **WHEN** a radar row's session has no attached client
- **THEN** the row carries a neutral marker saying no window is showing it — not an error
  colour, since it is a location, not a failure
