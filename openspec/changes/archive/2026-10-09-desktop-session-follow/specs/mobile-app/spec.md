## ADDED Requirements

### Requirement: Owner desktop follow form
Paired phone and iPad owners SHALL use a shared safe-area-aware, bounded per-conversation settings form. Saving SHALL use the read revision and prevent duplicate writes/dismissal. Server or conversation changes SHALL isolate late responses. Guests and incompatible cores SHALL NOT expose an actionable follow control.

#### Scenario: Conflict
- **WHEN** another device has updated the same policy
- **THEN** the save reports conflict without claiming success and offers reload

#### Scenario: Late response
- **WHEN** the user switches server while a request is pending
- **THEN** the old response cannot update another conversation
