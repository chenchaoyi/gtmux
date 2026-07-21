# Notifications Specification

## Purpose

Tell the user when an agent needs them (a permission/approval prompt) or finishes
its turn — by event timing, not message keywords — and deliver a desktop
notification that, when clicked, jumps to the agent. This is also the source of
the `waiting` and `latest` state the radar surfaces.
## Requirements
### Requirement: Hook state transitions by event timing

The system SHALL run as a hook (`gtmux hook`) on an agent's lifecycle events and
SHALL transition on-disk markers by event TIMING, never by message keywords:
`UserPromptSubmit` starts a turn, `Stop` ends it (records last-finished),
`Notification` marks waiting only mid-turn.

#### Scenario: Mid-turn notification is "needs you"

- **WHEN** a `Notification` event fires while a turn is active (active marker
  present)
- **THEN** the pane is marked `waiting` and a notification fires

#### Scenario: Idle nudge is not "waiting"

- **WHEN** a `Notification` event fires with no active turn (an idle nudge)
- **THEN** the pane is NOT marked waiting

#### Scenario: Turn finished

- **WHEN** a `Stop` event fires
- **THEN** active+waiting markers are cleared, the pane is recorded as
  last-finished, and a "finished" notification fires

### Requirement: Generic per-agent hook contract

The system SHALL accept `gtmux hook [--agent <key>] [<event>]`, mapping each
agent's raw events onto a canonical vocabulary (Claude's event names), so agents
beyond Claude Code (e.g. Codex's turn-complete) can drive the same behavior.

#### Scenario: Codex turn-complete

- **WHEN** Codex's notify runs `gtmux hook --agent codex` with its JSON payload
- **THEN** the `agent-turn-complete` event maps to the canonical `Stop`

### Requirement: Notification delivery via the app queue

The system SHALL deliver notifications through a queue the menu-bar app drains
(`internal/notify` writes JSON; the app posts native banners). There is no
terminal-notifier/osascript fallback — banners require the app running.

#### Scenario: Suppress when already viewing

- **WHEN** a notification would fire but the user is already viewing that
  session's terminal tab
- **THEN** the notification is suppressed

### Requirement: Install / uninstall the Claude hook

The system SHALL register/de-register `gtmux hook` in `~/.claude/settings.json`
idempotently (`install-hooks` / `uninstall-hooks`), and `doctor --fix` SHALL be
able to install it.

#### Scenario: Idempotent install

- **WHEN** `gtmux install-hooks` is run more than once (even from a moved binary)
- **THEN** the hook is registered exactly once, not duplicated

### Requirement: The supervisor is a meta-layer in notifications

The supervisor (HQ) session SHALL NOT be treated as a normal worker in the notification,
push, and lockscreen layers. The fleet tally that drives the lockscreen (the
Waiting/Working/Idle counts and the "who's waiting" headline) SHALL exclude the
supervisor, so HQ never inflates the worker counts nor hijacks the headline. The hook
SHALL suppress the supervisor's routine `done` notification (a supervisor finishing a
think-cycle must not notify the user); the supervisor's `input` notification (it needs a
decision from the user) SHALL be kept.

#### Scenario: HQ does not pollute the worker tally

- **WHEN** the serve fleet snapshot includes a `role:"supervisor"` pane that is waiting
- **THEN** the lockscreen tally's waiting count and "who's waiting" headline are computed from the worker panes only — the supervisor is excluded

#### Scenario: HQ's routine completion is silent

- **WHEN** the supervisor session finishes a turn (a `done`/Stop event) with the user not viewing it
- **THEN** no `done` notification is posted for it (unlike a worker), because a chief-of-staff completing a think-cycle is routine noise

#### Scenario: HQ still reaches you when it needs a decision

- **WHEN** the supervisor session emits an `input`/Waiting event (it needs the user's decision)
- **THEN** a notification is still posted, since that is the one thing the supervisor should surface

