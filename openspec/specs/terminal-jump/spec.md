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

