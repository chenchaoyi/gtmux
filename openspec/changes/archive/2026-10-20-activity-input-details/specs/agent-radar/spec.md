## MODIFIED Requirements

### Requirement: Desktop radar policy
Radar SHALL expose the saved per-conversation follow policy only for verified native Codex sessions in the configured home. Status-only rows SHALL remain visible in a separate External Codex group and SHALL be excluded from managed counts and urgency. Native Codex cwd SHALL NOT imply HQ role. Current control SHALL remain explicitly unconfirmed; creation metadata SHALL NOT be rendered as its current controller. Other-home observations SHALL not borrow titles/status from the configured home.

#### Scenario: Unselected desktop
- **WHEN** a verified desktop session has no policy
- **THEN** its follow fields are false, it is not adoptable or HQ, and detection does not create managed attention

#### Scenario: Other clients
- **WHEN** a terminal Codex or other agent is detected
- **THEN** its existing grouping and behavior are retained

#### Scenario: Shared Codex backend emits a transient hook ID

- **WHEN** an external Codex record in the configured home has a live shared backend PID but no matching persisted rollout
- **THEN** it does not appear as an external conversation, and its stored observation is not deleted
- **AND** a later matching rollout can surface on the next gather; distinct actual thread IDs remain separate even when their PID or directory matches
