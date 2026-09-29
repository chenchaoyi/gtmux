## MODIFIED Requirements

### Requirement: Sense agent sessions running outside tmux

For a native Codex conversation, the system SHALL treat all rollout files whose
session metadata confirms the same conversation ID as one lifecycle. It SHALL
select the latest turn boundary by event timestamp, without accepting a
lookalike filename from another conversation.

#### Scenario: Codex completes in a resumed rollout
- **WHEN** a native Codex record is working and its conversation continues in a rollout named with the same session ID plus an instance suffix
- **THEN** the latest turn boundary across matching rollouts SHALL determine whether the session is working or idle, after verifying the suffixed rollout's own session metadata
- **AND** a later `task_complete` or `turn_aborted` SHALL make it idle, while a later `task_started` SHALL keep it working
- **AND** a lookalike file belonging to another session SHALL NOT affect its status

#### Scenario: Conversation activity spans rollouts
- **WHEN** a Codex conversation has more than one matching rollout
- **THEN** its first and last activity times and Chat turns SHALL cover the matching rollouts in chronological order
- **AND** the Chat cache validator SHALL change when any matching rollout grows
