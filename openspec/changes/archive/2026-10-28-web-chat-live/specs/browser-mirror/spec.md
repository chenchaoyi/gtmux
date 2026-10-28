## ADDED Requirements

### Requirement: Collapsible working conversation preview
Chat SHALL show a collapsed-by-default Live strip while the current agent is working, shared by focused conversations and workspace panels. Expanding SHALL show the latest 14 non-trailing-blank rows from the current screen capture, not an earlier transcript reply. The strip SHALL retain its disclosure preference per pane across polling and mode switches, preserve drafts and reading position, and disappear when work stops. Runtime SHALL use the recorded current-state start and server clock when available, and be omitted when unknown. Capture errors SHALL identify unavailable or stale output and retry. Late responses SHALL not appear in a different pane, mode, turn or credential scope, and revoked access SHALL clear cached Live content.

#### Scenario: Watch an active turn
- **WHEN** the reader expands Live during a working conversation
- **THEN** current output updates without replacing transcript history or changing the draft, and the strip closes only when the reader chooses or work stops

#### Scenario: Switch sessions during a slow capture
- **WHEN** a capture response arrives after the reader switches to another session
- **THEN** that response cannot populate the new session's Live content
