# agent-digest Specification

## ADDED Requirements

### Requirement: Digest marks an input-locked pane

The `gtmux digest --json` / `GET /api/digest` contract SHALL carry an additive,
optional `in_mode` boolean, mirroring the radar: true when the pane is in tmux
copy-mode / view-mode (input-locked), absent (omitempty) otherwise. This lets the
supervisor see at a glance which pane is currently swallowing input, distinct from its
working/waiting/idle status.

#### Scenario: A digest row reflects the input lock

- **WHEN** a pane is in copy/view-mode and a client reads `gtmux digest --json` or
  `GET /api/digest`
- **THEN** that pane's digest row carries `in_mode:true`
- **AND** rows for panes not in a mode omit `in_mode`
