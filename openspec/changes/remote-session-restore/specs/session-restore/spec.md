## ADDED Requirements

### Requirement: Explicit remote recovery with durable progress
The server SHALL allow only an owner to preview missing saved sessions and explicitly start one headless restore job. Reads, reconnection and UI reopening SHALL NOT start recovery. Existing running/detached sessions SHALL remain untouched by remote agent startup. A stale or unreadable preview SHALL refuse before mutation.

#### Scenario: Connection lost after start
- **WHEN** the owner loses the HTTP response or closes the App
- **THEN** the persisted request and one claimed worker remain authoritative
- **AND** later reads or retries with the same ID return its state without executing again

#### Scenario: Startup is not confirmed
- **WHEN** a restore command was sent but the exact agent identity has not been observed
- **THEN** progress reports pending confirmation, never confirmed success

#### Scenario: Existing progress is readable without a new preview
- **WHEN** a paired pending request ID already exists
- **THEN** the client reads that exact job independently of a new workspace preview
- **AND** a failed read keeps the ID and exposes a read-only status retry without automatically starting work

### Requirement: Visible menu restore progress
The menu SHALL keep its progress visible after Restore is chosen, without overriding explicit dismissal or stealing focus.

#### Scenario: Restore click
- **WHEN** Restore begins
- **THEN** the generic menu action close does not hide the progress
- **AND** an explicit outside click or Escape still closes the menu
