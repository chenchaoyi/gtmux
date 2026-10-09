## ADDED Requirements

### Requirement: Desktop HQ attention enrollment
HQ unread debt, default event pulls, attention checks and history SHALL use the same desktop observation gate. Unselected conversations and events from before the current follow interval SHALL NOT create HQ debt. HQ SHALL NOT opt in conversations on its own.

#### Scenario: Default attention
- **WHEN** a desktop conversation emits lifecycle events with no enrollment
- **THEN** the raw ledger retains metadata but HQ debt and default pulls exclude it

#### Scenario: Observation interval
- **WHEN** the user stops and later reenrolls a conversation
- **THEN** only activity in the new interval is observed and prior records are retained
