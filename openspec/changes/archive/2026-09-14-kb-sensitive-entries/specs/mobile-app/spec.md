## ADDED Requirements

### Requirement: The knowledge sheet marks a sensitive entry

A knowledge entry the API marks `sensitive` SHALL show "sensitive" on its row and in its
axes line, in the reader's language.

#### Scenario: A sensitive entry on the phone

- **WHEN** the knowledge sheet lists an entry with `sensitive: true`
- **THEN** the row's meta line starts with "sensitive ·" (「敏感 ·」) and the detail's axes
  line says it stays on the Mac
