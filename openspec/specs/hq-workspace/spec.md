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

The workspace MAY deliver to a working HQ when the verified input route accepts it and the composer is empty. Queue acceptance SHALL be distinct from execution. Explicit acknowledged retry of uncertain delivery SHALL create a new transport generation; ordinary drains SHALL retain the same idempotency identity.

#### Scenario: Busy HQ and process failure
- **WHEN** HQ is working with an empty composer and a supported guarded input route
- **THEN** the instruction MAY be accepted into its queue without claiming execution
- **WHEN** HQ has an input draft or waiting prompt
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
The Mac SHALL expose Overview, knowledge, machine and usage in the existing HQ
workspace. The first opening and the menu-card board shortcut SHALL select Overview;
opening without a destination after navigation SHALL retain the current selection.
The complete authored situation board SHALL remain available as a dated, read-only
Handoff snapshot from Overview, with no competing live attention list or active links
to historical pane numbers. Knowledge SHALL retain search, list/detail navigation,
judgment actions, archive export, backup restore and selective migration. Overview
and resource destinations SHALL NOT expose an instruction composer or accept a send.
Switching away from a conversation SHALL preserve its draft.

#### Scenario: Open resources from the menu card
- **WHEN** the user opens the board shortcut
- **THEN** the same retained workspace selects Overview
- **WHEN** the user opens knowledge, machine or usage
- **THEN** that resource is selected in the same window
- **AND** reopening HQ without a destination preserves the selection

#### Scenario: Return to a draft
- **WHEN** a user switches from an unsent matter/global/new-task draft to Overview or a resource and back
- **THEN** the original draft is retained and no instruction is sent

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

### Requirement: Current work has one presentation source
The sidebar SHALL group explicit matter records by Needs your input, Following and
Recently ended. Overview SHALL use those same records for a bounded brief rather than
repeat the complete directory. Reads SHALL NOT infer state, links or goals from the
board. Each sidebar category SHALL have an independently operable full-row disclosure,
a count and accessible expansion state, retained within the workspace lifetime.
The Resources category SHALL precede all matter groups, below the primary navigation.
The sidebar column SHALL remove the framework-provided sidebar toggle, leaving only
the shared leading toolbar control in both expanded and collapsed layouts.
Matter glyphs SHALL reuse the session status language: waiting red/pause, following
brand cyan/ring, ended green/check. State labels SHALL remain available without color.

#### Scenario: Handoff text disagrees with registered matters
- **WHEN** a board still describes an old pending decision
- **THEN** current counts and actions come only from explicit matter state
- **AND** the full original board remains readable as a dated snapshot without hiding its content

#### Scenario: Collapse and reopen
- **WHEN** a user collapses one sidebar category and navigates or reopens the workspace
- **THEN** its disclosure choice is retained without collapsing other categories or losing selection

### Requirement: Loading is distinct from empty and failed content
Initial workspace, activity, resource and source-conversation reads SHALL use a gtmux
brand loading state until valid data or a failure arrives. A refresh SHALL retain valid
content with compact brand feedback. Errors SHALL expose retry without presenting an
unread conversation as empty or an unread supervisor as offline. Leaving a destination
SHALL NOT allow a late response to replace another destination's content.

#### Scenario: Slow or failed first read
- **WHEN** an initial read is pending
- **THEN** a brand loading view is shown instead of empty history or an offline claim
- **WHEN** it fails
- **THEN** a retryable failure is shown, distinct from a successfully empty result

### Requirement: HQ chat follows the app conversation conventions
HQ and linked-source conversation readers SHALL share avatars and sender names above
readable Markdown bubbles, with Copy full text below each public message and temporary
accessible copy confirmation. Copy SHALL preserve the complete source message.
Time separators SHALL appear at the first valid source time, a calendar day change or
a gap of at least 15 minutes from the previous valid source time. Missing/invalid times
SHALL NOT produce fabricated dates or reset the comparison. Delivery receipts,
source seams and folded processing details SHALL retain their existing meaning.

#### Scenario: A pause in conversation
- **WHEN** successive known timestamps are less than 15 minutes apart on the same day
- **THEN** no repeated timestamp is added
- **WHEN** the gap reaches 15 minutes or crosses midnight
- **THEN** the next message has a date and time separator

#### Scenario: Copy a formatted reply
- **WHEN** a user copies a public message from below its bubble
- **THEN** its full Markdown source is placed on the clipboard and completion is indicated
- **AND** the message scope and delivery state are unchanged

### Requirement: Source details stay within the workspace
Participant source readers and management sheets SHALL remain bounded by the
presenting workspace, independent of transcript length or loading state. Header
and Done/Escape SHALL remain available outside scrolling history. Source citations
SHALL open separately without expanding the matter header beyond its container.

#### Scenario: Long linked history finishes loading
- **WHEN** a source reader loads many long turns in a small workspace window
- **THEN** only its history scrolls and the reader's dimensions remain bounded
- **AND** the close action remains available without sending any instruction

### Requirement: Conversation entry reveals current content without disrupting reading
Opening or reselecting Talk to HQ or a matter SHALL reveal the newest loaded conversation, including cached content. Refresh while deliberately reading older messages SHALL preserve the reading position; a visible new-message action SHALL return to the latest content. Earlier-page loading and drafts SHALL retain their existing scope and content. Explicit latest-position requests SHALL tolerate lazy-row measurement changes within a bounded, navigation-cancelled layout window. They SHALL not run for ordinary historical refresh, and the latest-message action SHALL remain available if the bound is reached. Composer destination labels SHALL name HQ, the new task or the actual matter without internal routing prose or implying that matter association is impossible.

#### Scenario: A cached conversation is opened again
- **WHEN** the selected conversation already has loaded history before its view appears
- **THEN** the view opens at its newest loaded content
- **WHEN** the user scrolls into older history and refresh adds a reply
- **THEN** the reading position is retained
- **WHEN** the user clicks the same conversation destination again
- **THEN** the latest loaded content is revealed without discarding the draft or history

### Requirement: Workspace chrome remains stable across loading and sidebar changes
The leading toolbar SHALL keep Sidebar, Refresh and Open in Terminal at stable
positions when hiding/showing the sidebar, refreshing and resizing the window.
Refresh SHALL replace its existing icon with compact brand progress rather than
insert a new item, retain readable content, and use the selected workspace or
resource read state. It SHALL be disabled during that read or a conflicting action.
Sidebar category children, including Resources, SHALL be visibly indented beneath
their full-row headings without changing independent disclosure or selection state.

#### Scenario: Refresh with the sidebar hidden
- **WHEN** the user hides the sidebar and refreshes cached content
- **THEN** the same leading controls remain in place and refresh changes only its own mark
- **AND** content stays visible and duplicate refresh is unavailable during the read

#### Scenario: Collapse an indented category
- **WHEN** a selected child is inside a category the user collapses
- **THEN** only that category's children disappear and the selected destination remains unchanged

### Requirement: Overview is a bounded brief with explicit provenance
The workspace list SHALL accept an opt-in `--overview` projection built from the same loaded database. Ordinary list output SHALL omit assessment bodies. Overview SHALL show at most three needs-input cards, three following cards and five recent changed matters. Cards SHALL preserve the latest HQ message ID, kind and recorded clock with at most 768 Unicode characters and a truncation flag. A fallback saved assessment SHALL NOT inherit the matter update time as its report time or wait start. Reading SHALL NOT invoke a model, parse event logs/transcripts, mutate records or consume HQ's event watermark.

Current worker status SHALL require exact agent/home/session identity and an ongoing execution association. Idle or a result report SHALL NOT complete a matter. Full-category selection and card selection SHALL open original matter scope without sending an instruction. Wide layouts SHALL place the two brief categories side by side and narrow layouts SHALL stack them. The full sidebar directory and compact handoff entry SHALL remain available.

Recent changes SHALL use explicit state transitions, results, decisions, errors, unconfirmed delivery, registration, merges and splits within the recorded last 24 hours. They SHALL omit ordinary progress, delivery receipts and supervisor wake/turn noise, deduplicate by canonical matter and retain a count of changed matters rather than raw events. Missing clocks, future records and broken identity edges SHALL NOT create recent entries.

#### Scenario: A quiet worker still has unfinished work
- **WHEN** an exact linked execution conversation is idle but its matter remains open
- **THEN** the card reports the current conversation state without completing or reclassifying the matter

#### Scenario: Long history and reports
- **WHEN** the database has many ended matters and long multilingual reports
- **THEN** the default brief remains bounded, excerpts preserve Unicode and original source metadata, and View all keeps the full explicit category reachable

#### Scenario: No report clock exists
- **WHEN** a matter has only an assessment without its source message
- **THEN** the note is labelled as a saved assessment with no invented clock or wait duration

### Requirement: Conversation management starts with identifiable work
The native matter header SHALL open Conversations & records. Its default sheet SHALL name the current matter and list its included agent conversations, showing task title, agent, current terminal state only on exact identity match, and continuous or bounded record scope. A missing current terminal SHALL NOT be labelled proof of process termination. Status-since clocks SHALL NOT be presented as last-message times. Default content SHALL NOT expose split-selection checkboxes, time editors or destination pickers.

#### Scenario: A past record and an idle terminal are included together
- **WHEN** one included conversation has an exact current idle terminal and another has only saved records
- **THEN** their live availability is visibly distinct from their collection roles and time ranges
- **AND** neither an idle state nor the end of a collection range completes the matter or stops an agent

### Requirement: Record management is organized by intended action
Adding SHALL first offer identifiable current conversations and saved records with their existing matter relationships. A saved-only source SHALL be added as a bounded historical reference. Ongoing collection SHALL explicitly default to now with an editable start; future or inverted intervals SHALL be rejected locally. Individual record range and move editors SHALL be separate from whole-matter organization. Moving one conversation SHALL NOT preselect the destination of a later whole-matter merge.

Removal, individual move, split and merge SHALL require confirmation naming the affected scope and destination where applicable. Removal SHALL retain original messages and HQ discussion. A split SHALL move only selected associations and retain prior HQ discussion in the source. A merge SHALL explicitly say it collects all conversations and HQ discussion and closes the source matter. Editors and confirmations SHALL retain their captured revision and immutable identity; a failed save SHALL preserve the draft and display the error. Management SHALL NOT send instructions or drain the delivery queue.

#### Scenario: Review a move, then organize the whole matter
- **WHEN** a user chooses a destination for one conversation and returns to organize the matter
- **THEN** the whole-matter destination starts unselected
- **AND** confirming an individual move affects only that association and its existing range

#### Scenario: A historical source is added and the save fails
- **WHEN** a user selects a saved-only conversation and the metadata save fails
- **THEN** the chosen identity and valid bounded reference draft remain available with the error
- **AND** no instruction is sent to the agent or HQ
