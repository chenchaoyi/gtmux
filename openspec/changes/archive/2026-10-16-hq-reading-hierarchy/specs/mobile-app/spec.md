## ADDED Requirements

### Requirement: Settings and remote keys provide distinct readable symbols
Settings SHALL distinguish connection status, switching servers and the local HQ records copy using existing outline symbols matching their meaning. The phone and iPad SHALL share this presentation.

Composer Up, Down, Return and Backspace SHALL use legible 24pt symbols within targets of at least 44×44pt and expose spoken names. Key order, sent tmux key values, disabled behavior and local tap feedback SHALL be preserved.

#### Scenario: A reader uses a remote key
- **WHEN** the reader taps an enabled symbol key
- **THEN** its existing tmux key value is emitted once and the existing brief acknowledgment appears without shifting the row
- **AND** the symbol is readable inside a target of at least 44×44pt
