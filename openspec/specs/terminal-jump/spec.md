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
