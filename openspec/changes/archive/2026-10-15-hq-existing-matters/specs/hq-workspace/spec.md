## ADDED Requirements

### Requirement: Existing goals can be enrolled without reissuing instructions
HQ SHALL register a verified existing goal with a stable ID, typed source reference and
initial public assessment. Registration SHALL be restricted to the current HQ caller and
atomic, idempotent for identical intent, and reject conflicting reuse. It SHALL neither
create a user instruction nor trigger dispatch/delivery. HQ SHALL reconcile continuing work
on startup and maintain explicit matters across workspace, terminal and observed user channels.
Reads SHALL NOT create inferred matters or modify board/knowledge/transcripts.

#### Scenario: Upgrade with an existing board and no matter records
- **WHEN** the workspace has no registered matters
- **THEN** it explains that existing work needs HQ review and exposes the current board
- **AND** preparing a review request neither sends it nor overwrites a draft
- **WHEN** HQ verifies and registers an existing goal
- **THEN** its assessment and source are visible without repeating the original task

#### Scenario: Registration receipt is lost
- **WHEN** HQ retries the same registration after later progress or renaming
- **THEN** it returns the existing matter without resetting progress or adding a delivery
- **AND** conflicting reuse fails without partial writes
