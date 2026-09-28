# Notifications (delta)

## ADDED Requirements

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
