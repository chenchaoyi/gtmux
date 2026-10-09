## ADDED Requirements

### Requirement: Desktop notification consent
Desktop alerts SHALL require both follow and notification consent. Status-only rows SHALL NOT contribute to managed Live Activity or badge totals. Enrollment SHALL NOT replay existing waiting/done state. Delivery SHALL recheck consent and SHALL preserve session-specific identity without offering input.

#### Scenario: Independent notification
- **WHEN** HQ follow is enabled with notifications off
- **THEN** no waiting or completion push is emitted

#### Scenario: Revocation
- **WHEN** an alert is queued before permission is disabled
- **THEN** delivery refuses it

#### Scenario: Other agents
- **WHEN** a new terminal pane appears waiting
- **THEN** the existing waiting alert behavior is retained
