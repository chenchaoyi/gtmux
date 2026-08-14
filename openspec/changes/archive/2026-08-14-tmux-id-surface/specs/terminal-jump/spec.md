# terminal-jump (delta)

REWRITTEN 2026-08-14 after measuring a real tmux. The first draft of this delta specified
the ACTIVE pane's `%N` in `set-titles-string`, matchers updated in lockstep, and pane-border
labels as a `doctor` suggestion. All three were wrong, and the measurement is recorded in
the proposal's "Decided by measurement". What SHIPPED is below.

## ADDED Requirements

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
