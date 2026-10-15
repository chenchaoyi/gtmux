## ADDED Requirements

### Requirement: Bounded browser polling

The browser SHALL coalesce overlapping periodic reads for the same resource, view and credential. Read deadlines SHALL release the polling slot, and changing view or credential SHALL cancel obsolete reads. Mutation requests SHALL not be coalesced or automatically retried.

#### Scenario: A tunnel stops answering a poll

- **WHEN** a periodic request does not complete before its deadline
- **THEN** the request is cancelled, its slot is released, and a later poll can recover
