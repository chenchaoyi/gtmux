## ADDED Requirements

### Requirement: HQ workspace provides readable typography and classified activity
The full HQ window SHALL use a scoped reading type scale rather than compact menu-popover sizes: body and record titles at 15pt, sidebar labels at 14pt, metadata/code at least 13pt, and larger section/page headings. Sidebar category names SHALL not be smaller or fainter than child navigation labels. Markdown, native input and resource pages SHALL follow the same hierarchy without changing unrelated windows.

Existing action kinds SHALL be classified as instruction delivery, updates, matter/conversation organization, supervision, knowledge or maintenance, with an Other fallback. Rows SHALL use semantic symbols, expose the category in accessible text, and support filtering the loaded records by category without reordering their chronology or mutating stored data. Unknown kinds SHALL remain available in All and Other. Failure outcomes SHALL remain distinguishable from successful historical records.

#### Scenario: A reader filters activity
- **WHEN** the reader selects an activity category
- **THEN** matching workspace and journal records remain in their existing source order and the loaded count is shown
- **AND** returning to All restores every record, including unknown kinds

#### Scenario: Reading a full HQ window
- **WHEN** the reader opens the sidebar, conversation, resources or composer
- **THEN** the scoped reading sizes are used and the compact menu typography remains unchanged
