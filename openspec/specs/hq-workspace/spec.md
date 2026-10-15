# HQ workspace

## Purpose
Provide durable, explicitly scoped goals and HQ interaction, preserving source identity
and honest delivery receipts across supervisor turns and conversation changes.

## Requirements

### Requirement: Matters retain explicit goal and conversation identity
The workspace SHALL persist independent matter identifiers and explicit state. Multiple
conversations MAY participate, with role and bounded time interval. Membership SHALL
NOT be inferred from title, cwd or reused pane number. Manual association, move, unlink,
merge and split SHALL retain provenance without operating on agent clients.

#### Scenario: Multiple workers share a goal
- **WHEN** HQ dispatches workers with an explicit matter identifier
- **THEN** they appear under one matter with independent conversation identities
- **AND** completion of one turn does not complete the matter

#### Scenario: Historical reference and pane reuse
- **WHEN** a user associates a historical interval or its former pane is reused
- **THEN** only the selected original conversation interval is referenced
- **AND** the new occupant is neither linked nor controlled

### Requirement: Instructions have durable and honest delivery receipts
The workspace SHALL save scoped user instructions before attempting delivery, protect
drafts and waiting prompts, and separate delivery from execution. Duplicate IDs with
identical content SHALL converge; mismatching reuse SHALL fail. Uncertain attempts
SHALL NOT automatically replay. Failed reads or writes SHALL be visible.

#### Scenario: Busy HQ and process failure
- **WHEN** HQ is busy or has an input draft
- **THEN** the instruction remains pending without overwriting or submitting the draft
- **WHEN** a sender disappears after input may have occurred
- **THEN** the receipt is uncertain and is not blindly submitted again

#### Scenario: Automatic wake competes with an instruction
- **WHEN** another gtmux HQ input channel owns the input lease
- **THEN** the competing instruction or automatic wake remains queued without typing
- **AND** a later drain can deliver it after the lease is released

### Requirement: A modular native workspace exposes matters and HQ activity
The menu-bar HQ card SHALL open a sidebar/detail workspace. New task and global HQ
conversation SHALL be distinct from scoped matter replies. Matter headers SHALL show
associated conversations and management controls. User bubbles SHALL appear on the
right with the existing person-battery avatar and HQ replies on the left. Activity
SHALL expose receiving, processing, dispatch, maintenance and rotation evidence.

#### Scenario: Read without consuming HQ events
- **WHEN** a user opens the workspace, history or HQ activity
- **THEN** existing board, knowledge and transcript records are not rewritten
- **AND** HQ consumption watermark is unchanged

#### Scenario: New instruction and selected matter
- **WHEN** the user chooses New task
- **THEN** a new matter is created without silently appending to the selected matter
- **WHEN** the user sends from matter detail
- **THEN** the message is explicitly scoped to that matter and addressed to HQ

### Requirement: Existing goals can be enrolled without reissuing instructions
HQ SHALL register a verified existing goal with a stable ID, typed source reference and
initial public assessment. Registration SHALL be restricted to the current HQ caller and
atomic, idempotent for identical intent, and reject conflicting reuse. It SHALL neither
create a user instruction nor trigger dispatch/delivery. HQ SHALL reconcile continuing work
on startup and maintain explicit matters across workspace, terminal and observed user channels,
within existing HQ-follow consent. Native conversations without that consent SHALL NOT be enrolled.
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
