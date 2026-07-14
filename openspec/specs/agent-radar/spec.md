# Agent Radar Specification

## Purpose

Detect coding agents running inside tmux and report, at a glance, which are
waiting on the user, working, idle, or just running — plus where each lives and
the pane id to jump to. This is the single source of truth consumed by the CLI,
the menu-bar app, and the mobile app.
## Requirements
### Requirement: Detect agents inside tmux

The system SHALL detect coding-agent processes running inside tmux panes, and
SHALL NOT report a leftover agent title left on a pane that has returned to a
plain shell (a stale title is not a live agent).

#### Scenario: Live agent in a pane

- **WHEN** a tmux pane's foreground command is a known agent, or its process
  subtree invokes one (e.g. `node …/bin/codex` whose pane command is `node`)
- **THEN** the pane is reported as an agent with its display name resolved

#### Scenario: Stale title over a shell

- **WHEN** a pane shows a leftover agent title but no agent process is running
  (e.g. a resurrect-restored pane whose agent was never relaunched)
- **THEN** the pane is NOT reported as an agent

### Requirement: Classify agent status

The system SHALL classify each detected agent as `working`, `waiting`, `idle`,
or `running`, where `waiting` means blocked on the user (permission/approval).

#### Scenario: Working via title spinner

- **WHEN** a pane's title leads with an animating braille spinner glyph
- **THEN** the agent's status is `working`

#### Scenario: Working for a spinner-less agent

- **WHEN** an agent sets no title spinner (e.g. Codex) and its pane's visible
  content keeps changing between polls
- **THEN** the agent's status is `working`; if the content is static at a prompt,
  it is `idle`

#### Scenario: Waiting on the user

- **WHEN** a waiting marker exists for the pane (written by the notification
  hook) and the agent is not currently working
- **THEN** the agent's status is `waiting` and sorts to the top

#### Scenario: Waiting is never inferred from screen output

- **WHEN** a pane has NO hook waiting marker but its visible content contains a
  numbered list (e.g. a `1. … 2. …` list in the agent's own message)
- **THEN** the agent's status is NOT `waiting` — the waiting state comes from the
  hook/session only, never from parsing terminal output

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

### Requirement: Agent-agnostic profiles

The system SHALL identify agents by configurable profiles (command names + a
display label + optional idle glyph + icon), with built-ins for common agents
and user overrides via `~/.config/gtmux/agents.json`.

#### Scenario: User override wins

- **WHEN** `~/.config/gtmux/agents.json` defines a profile whose name matches a
  built-in
- **THEN** the user entry takes precedence

### Requirement: Radar includes non-tmux (native) sessions
The `gtmux agents --json` payload SHALL, in addition to tmux panes, include agent sessions sensed outside tmux as rows with `source: "native"`. The addition SHALL be backward compatible: existing consumers that ignore `source` (or treat unknown sources as non-focusable) MUST continue to work, and native rows MUST NOT carry a tmux-only focusable locator.

#### Scenario: Native rows carry source and no locator
- **WHEN** a client requests `agents --json` and native sessions exist
- **THEN** each native session SHALL be a row with `source: "native"`, agent/project/state/idle-time populated, and no focusable tmux locator

#### Scenario: Backward compatibility for tmux-only clients
- **WHEN** an older client reads `agents --json` containing native rows
- **THEN** the tmux rows SHALL be unchanged in shape and the client SHALL be able to skip native rows via the `source` field without error

### Requirement: Native rows are not focus/jump targets
The radar SHALL mark native rows as neither focusable nor send-able, so surfaces do not offer jump-to-terminal or reply on them.

#### Scenario: Focus is refused for a native session
- **WHEN** a focus/jump is attempted against a native session's identity
- **THEN** the system SHALL NOT attempt a terminal jump (there is no tmux/terminal locator for it)

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

