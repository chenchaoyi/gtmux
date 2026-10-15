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

### Requirement: The Mac has one HQ workspace window
The Mac SHALL expose situation board, knowledge, machine and usage as named sidebar destinations in the existing HQ workspace. Menu-card shortcuts and empty-state links SHALL select that destination in the same retained window. The legacy segmented HQ reader window SHALL NOT remain reachable. Knowledge SHALL retain search, list/detail navigation, judgment actions, archive export, backup restore and selective migration. Resource destinations SHALL NOT show an HQ instruction composer or accept a send. Switching away from a conversation SHALL preserve its draft.

#### Scenario: Open resources from the menu card
- **WHEN** the user opens knowledge, board, machine or usage from the menu card
- **THEN** the existing workspace is brought forward with that destination selected
- **AND** reopening HQ without a destination preserves the current selection

#### Scenario: Return to a draft
- **WHEN** the user switches from an unsent matter/global/new-task draft to a resource destination and back
- **THEN** the original draft is retained and no instruction has been sent

### Requirement: Resource reads follow workspace visibility
The Mac SHALL reuse bounded, gated resource reads and refresh only the selected resource destination. Closing or hiding the workspace, or leaving resources, SHALL stop its periodic resource reads. A late read from a prior destination SHALL NOT replace the selected destination's contents. Failures SHALL retain the last valid snapshot and refresh remains available.

#### Scenario: Knowledge followed by usage
- **WHEN** the user switches from knowledge to usage
- **THEN** usage is read without continuing knowledge polling
- **WHEN** the window closes
- **THEN** no resource timer remains active

### Requirement: Matter headers identify conversations without repeating replies
Matter headers SHALL show linked conversation names, agent, current pane/location and status when an exact current identity is available. Saved pane locators SHALL be labelled historical when no current identity matches, and SHALL NOT inherit another occupant's status. The header SHALL NOT repeat the HQ assessment above the full conversation. Existing stored assessments and source records SHALL remain intact.

#### Scenario: A linked worker exits and its pane is reused
- **WHEN** no current candidate matches the full linked conversation identity
- **THEN** its saved pane is shown as a last-known pane and its source remains readable
- **AND** another conversation occupying that pane does not supply the status or title
