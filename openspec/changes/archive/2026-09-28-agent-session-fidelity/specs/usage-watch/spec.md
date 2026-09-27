# usage-watch Specification Delta

## ADDED Requirements

### Requirement: Incremental counters are scoped by agent and session

The usage watcher SHALL key persistent incremental counters by the canonical agent key and
session id together. When the pair-keyed counter is absent, it SHALL read a legacy session-only
counter and write subsequent updates under the pair key.

#### Scenario: Two agents use the same session id

- **WHEN** two agents have sessions with the same opaque session id
- **THEN** their cumulative counters remain separate

#### Scenario: A legacy counter is present

- **WHEN** a pair-keyed counter is absent and the old session-only counter exists
- **THEN** gtmux reads the old counter and persists the next update under the pair key
