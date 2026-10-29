## ADDED Requirements

### Requirement: Bounded native Diff reading

Diff SHALL mount only a window of native rows while retaining access to every returned row and horizontal scrolling. Row metrics SHALL track system text size. Native line measurement SHALL be bounded; any shortened line SHALL have a visible notice. A failed request SHALL show an error and Retry, separately from an empty diff. Closing or switching panes SHALL retire earlier responses.

#### Scenario: Inspect a large working tree
- **WHEN** a diff contains 35000 lines
- **THEN** native row views remain bounded and the user can scroll, close and reopen the tool

#### Scenario: Retry a failed read
- **WHEN** the Diff read fails and the user selects Retry
- **THEN** the error remains distinct from no changes and a successful retry shows the current pane's diff
