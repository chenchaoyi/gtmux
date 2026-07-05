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

### Requirement: Stable JSON contract

The system SHALL expose the radar as `gtmux agents --json`: a byte-identical,
stable-shaped array consumed by all surfaces. Fields and their meaning are a
contract (see `internal/app/agents.go` `agentJSON`).

#### Scenario: Structured output

- **WHEN** a consumer runs `gtmux agents --json`
- **THEN** it receives a JSON array where each item carries at least `pane_id`,
  `session`, `window`, `pane`, `loc`, `agent`, `status`, `task`, `latest`,
  `activity`, `source`, and optional `icon`/`since`/`activity_at`
- **AND** an empty array when no tmux server is running

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

