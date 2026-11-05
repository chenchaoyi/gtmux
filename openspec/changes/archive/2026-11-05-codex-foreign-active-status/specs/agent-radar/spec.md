## ADDED Requirements
### Requirement: Foreign Codex active markers cannot revive a completed current thread
Radar SHALL correct a false working display only after validating the current thread identity and its own ended rollout plus ready nonworking terminal frame. It SHALL preserve unrelated markers and input authority.
#### Scenario: Another session marker survives
- **WHEN** a named marker belongs to another thread while the verified current thread has ended and is ready
- **THEN** recent window activity does not revive the current thread as working or create a new completion
#### Scenario: Ownership or readiness is uncertain
- **WHEN** binding is unavailable, the marker is unnamed/current, or current work/approval evidence exists
- **THEN** the foreign-marker correction does not suppress that evidence
