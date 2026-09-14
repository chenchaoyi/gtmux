## ADDED Requirements

### Requirement: The knowledge window marks a sensitive entry

A knowledge entry the API marks `sensitive` SHALL show a lock on its row and "sensitive ·
this Mac only" in its axes line; an attempt to promote it past `hq` SHALL show the CLI's
refusal verbatim.

#### Scenario: A sensitive entry in the list

- **WHEN** the knowledge window lists an entry with `sensitive: true`
- **THEN** its row carries a lock and its detail's axes line says it stays on this Mac
