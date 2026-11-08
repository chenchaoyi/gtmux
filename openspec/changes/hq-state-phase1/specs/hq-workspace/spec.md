## ADDED Requirements

### Requirement: Reports retain reconciliation responsibility
Reports SHALL persist observations without changing matter lifecycle or clearing user requests. Explicit state transitions SHALL validate the exact current matter revision. Question resolution SHALL match the current immutable question identity and revision. Legacy exact-ID replies MAY use that immutable identity as their question version.

#### Scenario: Late progress
- **WHEN** an old open report arrives after completion or a human question
- **THEN** the report is retained for reconciliation without reopening the matter or removing the question

### Requirement: Durable assessment receipts
The workspace SHALL persist a reconciliation queue independently of event consumption, fence expired lease holders, and atomically commit version-checked assessments with their receipts. Recovery and dry-run SHALL NOT send instructions or execute tasks. Source gaps and ambiguous ownership SHALL remain explicit.

#### Scenario: Supervisor replacement
- **WHEN** a lease expires and a new supervisor claims the observation
- **THEN** the old lease cannot commit and the new assessment is durable and idempotent
