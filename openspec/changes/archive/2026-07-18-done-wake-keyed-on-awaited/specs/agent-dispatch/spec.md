# agent-dispatch (delta)

## ADDED Requirements

### Requirement: A dispatch registers its target pane as awaited

The system SHALL register a dispatch's target pane as AWAITED — a durable marker that
HQ is expecting a completion from that pane — on both dispatch paths: `gtmux spawn`
after a delivered dispatch, and `gtmux send` on a confirmed landing (`landed`). The
unverified `POST /api/send` path SHALL NOT register (it is casual input, not an
HQ-awaited dispatch), and a delivery that does not land SHALL NOT register (no phantom
await). The awaited marker SHALL be cleared when the pane's completion wake fires or
when the pane goes away, so it is a one-shot per dispatch and leaves no stale state.

#### Scenario: A landed `gtmux send` marks the pane awaited

- **WHEN** `gtmux send` confirms its delivery landed on a pane
- **THEN** that pane is marked awaited, so its next completion wakes HQ immediately

#### Scenario: A failed delivery does not mark awaited

- **WHEN** a dispatch's delivery is not confirmed landed
- **THEN** the pane is NOT marked awaited — HQ is not left awaiting a phantom completion

#### Scenario: The await clears on completion

- **WHEN** an awaited pane's completion fires its `done` wake
- **THEN** the awaited marker is cleared, so a later unrelated completion does not
  re-fire an awaited wake unless HQ dispatches to the pane again
