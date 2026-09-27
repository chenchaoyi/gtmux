## ADDED Requirements

### Requirement: Knowledge mutations preserve their receipt identity

Each committed knowledge-ledger mutation SHALL carry an `op_id` shared with its
event and diagnostic audit receipts. The command SHALL report ledger write and
close errors to its caller.

#### Scenario: A lesson is added

- **WHEN** HQ adds a knowledge entry
- **THEN** the ledger operation, knowledge audit event and diagnostic act carry
  one matching operation ID
