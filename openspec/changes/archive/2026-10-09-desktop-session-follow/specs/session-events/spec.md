## ADDED Requirements

### Requirement: Desktop event permissions
Lifecycle records SHALL carry verified desktop client identity and independent knowledge permission metadata when available. Raw diagnostic records SHALL remain available; settings changes SHALL append metadata-only audit receipts.

#### Scenario: Receipt
- **WHEN** an owner successfully saves one conversation policy
- **THEN** the revision and permissions are recorded without conversation text
