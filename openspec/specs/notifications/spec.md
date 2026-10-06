# Notifications Specification

## Purpose

Tell the user when an agent needs them (a permission/approval prompt) or finishes
its turn — through typed hook events and lifecycle state, not message keywords — and deliver a desktop
notification that, when clicked, jumps to the agent. This is also the source of
the `waiting` and `latest` state the radar surfaces.

## Requirements

### Requirement: Hook state transitions by event timing

The system SHALL run as a hook (`gtmux hook`) on an agent's lifecycle events and
SHALL transition on-disk markers by event TIMING, never by message keywords:
`UserPromptSubmit` starts a turn, `Stop` ends it (records last-finished),
`Notification` marks waiting only mid-turn.

These are canonical lifecycle events after the per-agent classifier, not every raw
vendor event with the same name. For Claude's raw `Notification`, the implementation
accepts `permission_prompt`, `agent_needs_input` and `elicitation_dialog` as input
notifications; a missing `notification_type` retains the legacy mid-turn rule.
Other types, including `idle_prompt`, `auth_success` and `agent_completed`, are
telemetry and do not enter these transitions. This filtering has existed since
[#258](https://github.com/chenchaoyi/gtmux/pull/258); it does not remove the canonical
timing rules below or the later Codex-specific checks in this specification.

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

Here, firing a notification means requesting delivery through that queue after the
hook's suppression checks. A queued request is not proof that a banner appeared:
the app's notification setting, system authorization and stale-request filtering
also apply. `internal/notify/notify.go` produces requests;
`macapp/Sources/GtmuxBar/NotificationManager.swift` consumes them.

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

### Requirement: Codex completion banners require a verified pane

A Codex Stop event without a verified tmux pane SHALL remain in the lifecycle
journal but SHALL NOT post a desktop `done` notification. An attributed worker
completion SHALL retain its normal notification. This rule SHALL NOT suppress
HQ's `input` notification or native non-tmux Claude notifications.

#### Scenario: Shared app-server emits an unowned Stop

- **WHEN** a Codex Stop cannot be matched to a pane
- **THEN** the event is retained, no completion banner is queued, and diagnostics record the suppression reason

#### Scenario: Worker completion has a pane

- **WHEN** a Codex Stop is matched to an ordinary worker pane
- **THEN** the normal completion notification can be queued with that pane as its jump target

### Requirement: Codex approval alerts require a persistent user menu

Codex `PermissionRequest` SHALL be treated as an early observation, because it
can precede auto-review. It SHALL create a waiting marker and an input desktop
notification only if a numbered approval menu persists on the attributed pane
after a settling interval. An ownerless event SHALL not notify or mark a pane;
the radar MAY detect a live menu on its own pane later.

#### Scenario: Auto-review handles HQ permission

- **WHEN** Codex HQ emits PermissionRequest and continues without a persistent approval menu
- **THEN** no waiting marker or input notification is produced by the hook

#### Scenario: Codex needs a human decision

- **WHEN** an attributed Codex pane shows the same live approval menu after settling
- **THEN** the hook marks the pane waiting and may notify the user

### Requirement: HQ Codex does not bypass notification policy through its terminal

gtmux SHALL launch a Codex HQ with its TUI notifications filtered to input
prompts, excluding routine completion. It SHALL not change the user's global
Codex configuration. An explicit HQ agent command that sets
`tui.notifications` SHALL take precedence.

#### Scenario: Codex HQ finishes a think-cycle in Ghostty

- **WHEN** HQ was launched by gtmux using the default Codex options
- **THEN** Codex does not emit a Ghostty terminal completion notification
