## ADDED Requirements

### Requirement: Codex observation must not acquire control
The system SHALL use validated read-only capabilities on trusted existing backends
or verified bounded stored-history reads. Viewing SHALL NOT start a backend, resume,
submit, change permissions, or acquire a writer. Pagination and omissions SHALL be
bounded and explicit. Terminal content SHALL remain actual terminal output rather
than a fabricated transcript rendering.

#### Scenario: Validated existing-backend observer becomes available
- **WHEN** a versioned public capability supports read-only attachment without backend startup or writer acquisition
- **THEN** isolated validation SHALL precede adoption of that observer
- **AND** stored-history reading SHALL remain the fallback when the capability is unavailable
