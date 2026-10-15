## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: Current work has one presentation source
Overview and sidebar SHALL group the same explicit matter records by Needs your input,
Following and Recently ended. Reads SHALL NOT infer state, links or goals from the
board. Each sidebar category SHALL have an independently operable full-row disclosure,
a count and accessible expansion state, retained within the workspace lifetime.
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
