## ADDED Requirements

### Requirement: DSH usage follows native settlement semantics

The system SHALL count DSH v4 provider-reported token buckets from native durable
settlements, replacing a repeated turn/step sample and counting explicit retry
attempts separately. Input and cache traffic SHALL remain distinct; reasoning
output SHALL NOT be added twice. Session and local-day totals SHALL reuse bounded
incremental reads, retain source identity and reject unsupported or ambiguous logs.
Native account observations SHALL never change lifecycle or receipt state.

#### Scenario: A settlement is restated and then retried

- **WHEN** a turn/step usage sample is updated and an explicit retry starts
- **THEN** the update replaces its earlier sample and the new attempt contributes once
