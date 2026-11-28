## ADDED Requirements

### Requirement: Bounded native Live scrollback layers
Mobile expanded Live terminal scrollback SHALL partition its up-to-1,000 captured lines into vertical `Text` chunks of at most 32 rows inside a column container, and SHALL wrap lines exceeding 320 terminal cells while keeping narrower rows unwrapped for horizontal scrolling, so each native text layer stays within GPU texture limits.

#### Scenario: Pane capture exceeds single-texture height on Retina displays
- **WHEN** a reader expands Live on a pane with 1,000 captured lines on a `@3x` phone or `@2x` tablet
- **THEN** the scrollback renders in stacked chunks of at most 32 rows without exceeding native backing-store limits
- **AND** wide terminal lines up to 320 cells scroll horizontally while pathological wider lines wrap without losing ANSI styling
