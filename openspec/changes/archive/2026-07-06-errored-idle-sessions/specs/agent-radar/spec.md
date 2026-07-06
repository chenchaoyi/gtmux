## ADDED Requirements

### Requirement: Mark idle sessions that ended on an error

The system SHALL distinguish an `idle` agent whose last turn ended on an API/tool
error from one that completed successfully, by reading the agent's transcript. An
errored idle session SHALL carry an `error` flag (and a short `error_text` summary)
so surfaces can mark HOW it ended. This is a modifier on the `idle` state — NOT a
new status — and MUST NOT be encoded with the `waiting`/needs-you color (red).

#### Scenario: Session ended on an API error

- **WHEN** an agent is `idle` and the last message of its Claude Code transcript is
  an assistant entry flagged `isApiErrorMessage: true` (e.g. `Unable to connect to
  API`, `response exceeded the … token maximum`, `Internal server error`)
- **THEN** the agent's row carries `error: true` and an `error_text` summary of the
  failure
- **AND** its status remains `idle`

#### Scenario: Transient mid-turn errors that recovered

- **WHEN** an agent's transcript contains `isApiErrorMessage` entries from earlier
  retries but its LAST message is a normal assistant/user entry (the turn recovered
  or continued)
- **THEN** the agent's row does NOT carry `error` (it completed normally)

#### Scenario: Non-idle or non-Claude agents

- **WHEN** an agent is `working`/`waiting`, or its transcript is unavailable/not a
  Claude Code log
- **THEN** the agent's row does NOT carry `error` (the flag only annotates idle
  sessions whose end can be read)

## MODIFIED Requirements

### Requirement: Stable JSON contract

The system SHALL expose the radar as `gtmux agents --json`: a byte-identical,
stable-shaped array consumed by all surfaces. Fields and their meaning are a
contract (see `internal/app/agents.go` `agentJSON`).

#### Scenario: Structured output

- **WHEN** a consumer runs `gtmux agents --json`
- **THEN** it receives a JSON array where each item carries at least `pane_id`,
  `session`, `window`, `pane`, `loc`, `agent`, `status`, `task`, `latest`,
  `activity`, `source`, and optional `icon`/`since`/`activity_at`/`error`/`error_text`
- **AND** an empty array when no tmux server is running

#### Scenario: Errored-idle fields are backward compatible

- **WHEN** an idle session ended on an error
- **THEN** its item additionally carries `error: true` and `error_text`
- **AND** for every other row `error` is absent/false, so a consumer that does not
  know the field behaves exactly as before
