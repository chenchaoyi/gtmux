## ADDED Requirements

### Requirement: Journal append failures are visible

The event append path SHALL surface filesystem and short-write errors through a
bounded stderr warning while keeping the triggering hook or action alive. An
explicitly disabled event journal is not a write failure.

#### Scenario: Journal path is unavailable

- **WHEN** an event cannot be appended because its file cannot be opened
- **THEN** the caller continues and stderr names the failed event store
