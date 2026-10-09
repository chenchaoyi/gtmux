## ADDED Requirements

### Requirement: Desktop radar policy
Radar SHALL expose the saved per-conversation follow policy only for verified native desktop Codex sessions. Status-only rows SHALL remain visible in a separate Desktop apps group and SHALL be excluded from managed counts and urgency. Desktop cwd SHALL NOT imply HQ role.

#### Scenario: Unselected desktop
- **WHEN** a verified desktop session has no policy
- **THEN** its follow fields are false, it is not adoptable or HQ, and detection does not create managed attention

#### Scenario: Other clients
- **WHEN** a terminal Codex or other agent is detected
- **THEN** its existing grouping and behavior are retained
