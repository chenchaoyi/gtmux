## ADDED Requirements

### Requirement: Terminal captures respect screen history boundaries

The shared pane capture used by phone, iPad and web SHALL select its history range
inside one tmux command queue. Normal screens SHALL include the existing bounded
scrollback; alternate screens SHALL expose only their current frame, never the
frozen normal-screen history. Capture SHALL preserve ANSI, physical rows, trailing
blank rows and bottom-relative cursor geometry, and SHALL NOT clear buffers,
restart clients or synthesize terminal history from conversation logs.

#### Scenario: An alternate-screen Codex follows older shell output

- **WHEN** normal history contains an old git file list and the active alternate screen contains a later Codex turn
- **THEN** the Terminal response contains only the active frame, without a false join to the old list
- **AND** after the program returns to its normal screen, real bounded scrollback remains available
