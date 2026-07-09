## MODIFIED Requirements

### Requirement: Stable JSON contract

The system SHALL expose the radar as `gtmux agents --json`: a byte-identical,
stable-shaped array consumed by all surfaces. Fields and their meaning are a
contract (see `internal/app/agents.go` `agentJSON`).

#### Scenario: Structured output

- **WHEN** a consumer runs `gtmux agents --json`
- **THEN** it receives a JSON array where each item carries at least `pane_id`,
  `session`, `window`, `pane`, `loc`, `agent`, `status`, `task`, `latest`,
  `activity`, `source`, and optional
  `icon`/`since`/`activity_at`/`error`/`error_text`/`bg`/`bg_count`/`bg_text`
- **AND** an empty array when no tmux server is running

#### Scenario: Errored-idle fields are backward compatible

- **WHEN** an idle session ended on an error
- **THEN** its item additionally carries `error: true` and `error_text`
- **AND** for every other row `error` is absent/false, so a consumer that does not
  know the field behaves exactly as before

#### Scenario: Background-running fields are backward compatible

- **WHEN** an idle session still has in-flight background work
- **THEN** its item additionally carries `bg: true`, `bg_count` (the number of
  in-flight background tasks), and `bg_text` (a short summary, e.g. the shell
  command line)
- **AND** for every other row `bg` is absent/false, so a consumer that does not
  know the field behaves exactly as before

## ADDED Requirements

### Requirement: Mark idle sessions with background work still running

The system SHALL distinguish an `idle` agent whose turn ended while background
work it started is still running, from one that is truly finished, by reading the
agent's own end-of-turn signal. A `Stop` hook payload that reports in-flight
background work SHALL cause the session's row to carry a `bg` flag (with
`bg_count` and a short `bg_text` summary) so surfaces can mark that it is "paused
waiting for background work", not done. This is a modifier on the `idle` state —
NOT a new status — and MUST NOT be encoded with the `waiting`/needs-you color
(red); it SHALL use an amber/neutral treatment, mirroring the `error` modifier.

Scope: this signal is read from Claude Code's `Stop` payload `background_tasks`
array (each item having a type such as `shell`/`subagent`/`monitor`/`workflow`
and, for shells, a `command`). Agents that expose no equivalent end-of-turn
signal (e.g. Codex) SHALL NOT carry `bg`.

#### Scenario: Turn ends with a background shell still running

- **WHEN** a Claude Code agent is `idle` and its `Stop` hook payload's
  `background_tasks` array holds at least one running/pending item (e.g. a
  `run_in_background` shell such as `npm run dev`)
- **THEN** the agent's row carries `bg: true`, `bg_count` equal to the number of
  in-flight items, and a `bg_text` summary of the work
- **AND** its status remains `idle`

#### Scenario: Turn ends with no background work

- **WHEN** a Claude Code agent is `idle` and its `Stop` payload's
  `background_tasks` array is empty (or absent)
- **THEN** the agent's row does NOT carry `bg` (the session is truly done)
- **AND** any previously recorded background-work marker for that pane is cleared

#### Scenario: Background work finishes on a later turn

- **WHEN** a pane previously marked `bg: true` produces a subsequent `Stop`
  payload whose `background_tasks` is empty (the background work completed and the
  session settled)
- **THEN** the pane's background-work marker is cleared and its row no longer
  carries `bg`

#### Scenario: Non-idle or signal-less agents

- **WHEN** an agent is `working`/`waiting`, or it exposes no `background_tasks`
  signal at turn end (e.g. Codex, or a non-Claude agent)
- **THEN** the agent's row does NOT carry `bg` (the flag only annotates idle
  sessions whose end-of-turn background state can be read)

#### Scenario: Surfaces render the modifier without red

- **WHEN** any surface (CLI, menu-bar, mobile, Web) renders a row carrying `bg`
- **THEN** it shows a "background running" modifier (e.g. `⧗N`) in an
  amber/neutral tone, keeping the `idle` status glyph/section
- **AND** it MUST NOT use the `waiting` red for the modifier
