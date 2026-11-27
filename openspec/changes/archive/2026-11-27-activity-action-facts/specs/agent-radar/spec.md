## ADDED Requirements

### Requirement: Recorded Activity action facts
The core SHALL provide optional bounded action and target-count metadata only from explicit recorded tool intent, without changing the original operation, call identity, raw input or detail reference. Consumers SHALL preserve real call counts.

#### Scenario: A recorded skill read
- **WHEN** an explicit skill method or a recorded read path under a named skills directory identifies a skill
- **THEN** consumers can show the skill identity without claiming skill application or reading extra files

#### Scenario: A known web operation
- **WHEN** a known tool or complete recognized arguments identify search, open or find
- **THEN** the shared Activity and Trace labels show that action
- **AND** mixed or unknown web operations keep a neutral label
