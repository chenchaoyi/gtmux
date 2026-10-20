## ADDED Requirements

### Requirement: Secondary tool activity opens only on reader intent

Phone and iPad Chat SHALL keep both live and historical tool groups folded by default as a grey summary on the reading canvas. Opening SHALL reveal compact icon-led rows without a separate card background. Only a call with recorded additional input, directory or a partial-detail notice SHALL offer detail expansion. Empty inspectors and repeated tool-name placeholders SHALL be omitted. Original input SHALL remain selectable, horizontally scrollable and copyable; bounded multiline scrolling and partial/summary-only notices SHALL remain. Reader choices SHALL survive updates and turn completion. Commentary and elapsed-time progress SHALL remain visible independently.

#### Scenario: Argument-free wait

- **WHEN** a reader expands a group containing only a Wait without recorded parameters
- **THEN** it shows one non-interactive Wait row, without an empty inspector or detail chevron

#### Scenario: Inspect a real command

- **WHEN** a reader expands recorded long command input
- **THEN** the displayed text can be scrolled and copied without executing it, and closing the group removes its detail views

#### Scenario: Live calls arrive

- **WHEN** calls append to a working turn
- **THEN** the summary updates without automatically opening the list or overriding a deliberate reader choice

## MODIFIED Requirements

### Requirement: Full reply actions

Phone and iPad Chat SHALL provide icon-only Copy reply and Share beneath each text-bearing turn, including when folded. Actions SHALL use every textual segment in order as Markdown, falling back to response for older transcripts, and exclude the prompt and tool output. In-progress replies SHALL use only text received so far. Copy SHALL show a temporary checkmark confirmation, and Share SHALL open the system share sheet without selecting a recipient. Actions SHALL retain localized accessibility labels and touch targets at least 44 by 44 points, without visible text labels. Tool-only turns SHALL not show these actions. The unboxed action row SHALL follow the final text segment before any trailing tool calls.

#### Scenario: Copy a folded segmented reply

- **WHEN** the reader copies a reply with hidden text segments
- **THEN** the clipboard contains all of that turn's textual segments in order, not its truncated preview
- **AND** the copy control confirms completion

#### Scenario: Share a reply

- **WHEN** the reader taps Share below a reply
- **THEN** the system share sheet receives the same complete text and the reader chooses the destination
