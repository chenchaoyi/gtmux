## ADDED Requirements

### Requirement: External groups can be folded
Both non-tmux groups SHALL provide a whole-header Show/Hide button with a chevron and retained count, matching the existing status-group interaction. Each group's collapsed choice SHALL persist independently of other groups and survive popover reopen. Collapsed groups SHALL omit their rows from the flattened list while keeping their header visible.

#### Scenario: Fold external sessions
- **WHEN** the user folds the external-session group
- **THEN** its header and count remain, its rows disappear, and status and elsewhere groups retain their own choices

#### Scenario: Reopen and unfold
- **WHEN** the popover is reopened after folding a non-tmux group
- **THEN** it stays folded until the user unfolds it
