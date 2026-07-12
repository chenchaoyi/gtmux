## MODIFIED Requirements

### Requirement: Stable JSON contract

The system SHALL expose the radar as `gtmux agents --json`: a byte-identical,
stable-shaped array consumed by all surfaces. Fields and their meaning are a
contract (see `internal/app/agents.go` `agentJSON`). Rows MAY carry an additive,
optional `role` field — currently the only value is `"supervisor"`, marking a
supervisor (中控) session detected by its pane cwd being the supervisor home; the
field is absent for normal agents so existing consumers are unaffected.

#### Scenario: Structured output

- **WHEN** a consumer runs `gtmux agents --json`
- **THEN** it receives a JSON array where each item carries at least `pane_id`,
  `session`, `window`, `pane`, `loc`, `agent`, `status`, `task`, `latest`,
  `activity`, `source`, and optional
  `icon`/`since`/`activity_at`/`error`/`error_text`/`bg`/`bg_count`/`bg_text`/`role`
- **AND** an empty array only when there are neither tmux agent panes NOR any live
  `source:"native"` session (a sensed native agent still appears with no tmux server
  running, since `gatherAgents` appends native rows after the tmux scan)

#### Scenario: Supervisor row carries role

- **WHEN** a supervisor session (see `supervisor-agent`) is live
- **THEN** its row includes `role:"supervisor"` and all other rows omit `role`
