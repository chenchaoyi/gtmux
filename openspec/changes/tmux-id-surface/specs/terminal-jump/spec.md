# terminal-jump (delta)

## ADDED Requirements

### Requirement: The terminal tab title carries a tmux id that maps to the app

The terminal tab title SHALL include a tmux id that the pane browser also shows, so a tab
can be bridged to an app row. Because a tab is a viewport onto its session's active
window's active pane, the title SHALL carry the ACTIVE pane's `%<pane_id>` (e.g.
`#S #{pane_id} #W`) — the pane is the unit of work and is what the supervisor (HQ) refers
to by `%N`. The session name and window name MAY remain in the title as human glosses.

When the tab title format changes, the terminal drivers' title MATCHERS
(`FocusTab`/`IsViewing`/`TabOrder`) SHALL be updated in lockstep so session-level tab
matching keeps working, and `doctor`'s expected `set-titles-string` SHALL be updated to
match.

#### Scenario: A tab maps to the pane HQ named

- **WHEN** HQ refers to pane `%11` and the user looks at their terminal tabs
- **THEN** the tab whose active pane is `%11` shows `%11` in its title, so the pane is found

### Requirement: Split-window panes wear their id on the pane border

An opt-in mode SHALL label each pane's tmux border (`pane-border-format` /
`pane-border-status`) with its `%<pane_id>`, so that in a split window every pane —
including the non-active one the tab title cannot name — shows its id on screen. Because it
reserves a row and changes the terminal's look, it SHALL be opt-in (suggested by `doctor`),
not forced.

#### Scenario: The non-active pane in a split is still identifiable

- **WHEN** a window is split into an agent pane `%2` and a shell pane `%28`, and the shell
  is active (so the tab title names `%28`)
- **THEN** with pane-border labels on, the agent pane's border shows `%2`, so a reference to
  `%2` is found without it being the active pane
