# mobile-app (delta)

## ADDED Requirements

### Requirement: A phone may preserve the Mac pane's original rows

The Detail terminal SHALL offer Wrap and Original width modes. Wrap SHALL remain
the default for narrow screens. Original width SHALL keep each captured terminal
row intact, preserve its ANSI colors and cursor position, and allow horizontal
panning without resizing the Mac pane. The mode SHALL use the pane's reported
columns; when an older server omits them, it SHALL use the widest captured row.
Both modes SHALL retain vertical scrollback and text selection.

#### Scenario: A wide Codex diff

- **WHEN** the user selects Original width on a phone viewing a wide Codex pane
- **THEN** each captured diff row remains one row with its colors, and the user
  can pan horizontally to read the rest

#### Scenario: Older Mac server

- **WHEN** the pane response has no column count
- **THEN** Original width still keeps captured rows intact using their content
