## ADDED Requirements

### Requirement: Codex HQ rotation confirms a durable handoff
A different in-memory thread ID SHALL establish switched state only. Codex rotation
completion SHALL additionally require same-home durable history and successor turn activity
after the sent reset. The existing startup briefing MAY continue the handoff once through
the shared guarded input channel, respecting quiet startup and protecting drafts. The
attempt SHALL be durable before delivery; uncertain input SHALL not be blindly retried.
The prior recovery generation SHALL not be discarded solely for a new unsaved ID.

#### Scenario: A new empty HQ has not received its handoff
- **WHEN** the frontend switches but no persisted first turn exists
- **THEN** the rotation remains pending and the existing startup channel continues it once when safe
- **AND** no second reset, fake user task or draft overwrite occurs

#### Scenario: A successor is ready
- **WHEN** its durable metadata and post-reset turn activity are verified
- **THEN** rotation records successful completion without further input

### Requirement: Conversation binding diagnosis respects direct evidence
Doctor SHALL prefer verified current frontend identity over newer files sharing a directory.
Directory recency alone SHALL not produce a repair recommendation or automatic rebind.
Legacy clients without independent proof SHALL be reported as informational uncertainty.

#### Scenario: A restored client shares its history directory with other sessions
- **WHEN** its own live frontend proves the exact current thread
- **THEN** unrelated newer history does not create a binding repair warning
