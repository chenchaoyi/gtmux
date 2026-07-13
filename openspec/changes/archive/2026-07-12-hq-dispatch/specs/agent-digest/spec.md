# agent-digest Specification

## ADDED Requirements

### Requirement: Digest rows surface a dispatched task's goal and status

A digest row whose pane has a dispatch-ledger entry (from `gtmux spawn`) SHALL
carry the dispatched task's goal and its lifecycle status (delivered → working →
waiting → done) as additive fields. Rows for panes with no ledger entry SHALL be
unchanged (the fields absent). The status SHALL be derived from the pane's live
radar state, consistent with `gtmux tasks`.

#### Scenario: A dispatched pane shows its task

- **WHEN** a pane was dispatched via `gtmux spawn` and `gtmux digest --json` runs
- **THEN** its row additionally carries the dispatched goal and lifecycle status

#### Scenario: Untracked panes are unchanged

- **WHEN** a pane was not dispatched via `gtmux spawn`
- **THEN** its digest row carries no dispatch fields (fully additive)
