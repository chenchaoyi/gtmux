## ADDED Requirements

### Requirement: Desktop content observation consent
Digest SHALL exclude status-only desktop conversations before invoking content or usage readers. Enrolled rows SHALL carry session ID, client and policy so HQ can distinguish observation from knowledge consent.

#### Scenario: Default
- **WHEN** a desktop conversation has not been enrolled
- **THEN** no conversation content is read by digest

#### Scenario: Observe
- **WHEN** the owner enables HQ follow without knowledge capture
- **THEN** digest may report its conversation but this is not authority to file its content
