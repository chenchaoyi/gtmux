## ADDED Requirements

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
