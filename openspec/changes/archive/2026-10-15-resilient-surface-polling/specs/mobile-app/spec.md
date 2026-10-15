## ADDED Requirements

### Requirement: Scoped periodic reads

The phone and iPad SHALL allow at most one periodic read per resource and view scope. Reads SHALL have a deadline, be cancelled on scope disposal, and never publish after disposal or invalidation. Failed reads SHALL retain the last valid snapshot; manual actions SHALL be able to invalidate a pre-action snapshot.

#### Scenario: A slow capture outlives a page change

- **WHEN** the reader leaves a pane while its capture is pending
- **THEN** the capture is cancelled and cannot replace data on the next pane
