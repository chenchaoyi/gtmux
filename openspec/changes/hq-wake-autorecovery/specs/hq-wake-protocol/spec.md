## ADDED Requirements

### Requirement: Attributable recovery conflicts preserve progress for independent wakes

Recovery SHALL preserve old intent and pin evidence and protect the exact affected queue location. Recovery errors that cannot identify a safe scope SHALL hold new submissions. The producer SHALL give each new physical entry a unique incarnation while coalescing only an eligible unclaimed entry of the same key.

#### Scenario: Two old incarnations collide during parking

- **WHEN** two owned regular files with different identities occupy the known claim and its parked destination
- **THEN** both remain and an independent due wake may be admitted
- **AND** neither conflicting entry is replayed
