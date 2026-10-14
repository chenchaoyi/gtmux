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
