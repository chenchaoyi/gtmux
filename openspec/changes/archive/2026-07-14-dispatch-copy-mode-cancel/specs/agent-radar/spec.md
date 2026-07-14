# agent-radar Specification

## ADDED Requirements

### Requirement: Radar marks an input-locked (copy-mode) pane

The `gtmux agents --json` contract SHALL carry an additive, optional `in_mode`
boolean: true when the tmux pane is currently in copy-mode / view-mode (the user
scrolled the scrollback), where typed input — manual or programmatic — is swallowed
as mode-navigation until the pane exits the mode. The field SHALL be absent
(omitempty) for panes not in a mode and for non-tmux (`source:"native"`) rows, so
existing consumers are unaffected. It lets a surface show WHICH pane is input-locked;
`gtmux send`/`spawn` themselves auto-exit the mode before delivering.

#### Scenario: An input-locked pane is flagged

- **WHEN** a tmux agent pane is in copy/view-mode and a consumer runs
  `gtmux agents --json`
- **THEN** that pane's row carries `in_mode:true`
- **AND** rows for panes not in a mode omit `in_mode`
