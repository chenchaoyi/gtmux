## ADDED Requirements

### Requirement: Automatic HQ input channels serialize their attempts
Workspace instructions, automatic wakes, conversation rotation and startup briefing
SHALL share a nonblocking input lease while checking and typing. Contended durable
requests SHALL remain saved; a best-effort startup briefing SHALL report refusal.
Rotation SHALL re-read its durable attempt after acquiring the lease so an obsolete
unsent snapshot cannot repeat a reset. This lease SHALL NOT replace draft guards.

#### Scenario: Rotation competes with a saved instruction
- **WHEN** another HQ input channel owns the lease
- **THEN** rotation sends no keys and retains its request
- **WHEN** a later processor receives an old unsent snapshot of an already sent request
- **THEN** it uses the saved attempt and does not send the reset again
