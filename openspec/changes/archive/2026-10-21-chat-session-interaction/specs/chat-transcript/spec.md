## ADDED Requirements

### Requirement: Conversations expose live terminal interaction
A managed tmux conversation SHALL expose pending input even when numbered choices cannot be parsed. It SHALL provide a bounded current-screen view and explicit keyboard input without requiring a switch to Terminal. Manual inspection SHALL be available without inferring a wait. Scope and input permissions SHALL remain authoritative. Failed/stale reads SHALL not leave old choices actionable; external read-only conversations SHALL not gain input.

#### Scenario: Unrecognized picker
- **WHEN** a waiting tmux conversation has no safe numbered choices
- **THEN** Chat shows a current-screen preview and explicit keys for the reader to operate it
- **AND** no choice or default is invented

#### Scenario: Missing wait signal
- **WHEN** the reader manually opens interaction in a managed conversation
- **THEN** the current screen and permitted input remain available without changing agent status
