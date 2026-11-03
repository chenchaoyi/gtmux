## ADDED Requirements

### Requirement: Fresh single-pane agent status

The CLI SHALL support `agents --json --pane %N` with the same fresh pane identity and status pipeline as the full radar. It SHALL skip unrelated pane histories and native-session enumeration, and SHALL NOT reap orphan markers from a partial scan. The option SHALL reject invalid pane IDs and incompatible watch/popup or non-JSON modes. A missing pane SHALL return an empty array.

#### Scenario: A screenshot needs a current permission verdict

- **WHEN** the menu-bar screenshot sender checks its chosen pane
- **THEN** it reads only that pane through the shared pipeline and retains session-name, waiting, strict-screen and draft protections
