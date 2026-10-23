## ADDED Requirements

### Requirement: Restore displays a read-only pane-ID receipt

After loading a saved layout and reading live windows, restore SHALL list each saved
session/window with its saved and current pane IDs. It SHALL extract IDs from layout
leaf positions, never infer their order from numeric IDs or pane indices. Individual
backup-to-current pairs SHALL require identical normalized arrangements and complete,
unique ID sets, except that one saved and one current pane in the same window is
unambiguous even after a resize. Otherwise it SHALL list both sets separately and
state that individual correspondence is unknown. Missing windows and legacy layouts
without IDs SHALL be explicit. The receipt SHALL NOT affect input, resume, cleanup or
layout-changing decisions; the read-only plan JSON remains unchanged.

#### Scenario: Matching layouts with renumbered panes
- **WHEN** the saved and live layouts have identical geometry and unique pane IDs
- **THEN** each old ID is paired with the current ID at that same layout position

#### Scenario: A changed window cannot be paired safely
- **WHEN** a multi-pane window has a different count or normalized arrangement
- **THEN** both ID sets remain visible without guessed individual pairs

#### Scenario: Old evidence is unavailable
- **WHEN** an older saved window has no readable pane IDs
- **THEN** the receipt names the missing evidence and still shows available current IDs
