## MODIFIED Requirements

### Requirement: Read-only grouped health check

The system SHALL announce each check section on stderr before evaluating that
section when running in a terminal. `--progress` SHALL enable the same announcements
when output is redirected. The optional Homebrew update suggestion SHALL NOT delay
the report indefinitely.

#### Scenario: A probe stalls

- **WHEN** a doctor probe is slow
- **THEN** the last announced section identifies the current stage
- **AND** a stuck Homebrew update lookup times out, allowing the report to finish
