## ADDED Requirements

### Requirement: Historical launch-host display evidence
The core SHALL optionally record a native session's launch host on SessionStart, UserPromptSubmit or Resumed hooks, before detaching a worker. It SHALL inspect only a bounded ancestor chain, read process identifiers, parent identifiers, birth times and executable names without full command arguments, prefer a macOS app display name, and otherwise use an executable basename. Uncertain, changed, timed out, interpreter-only or service-boundary observations SHALL remain unknown.

#### Scenario: Generic wrapper hosts a native agent
- **WHEN** a recognized agent ancestor has a stable, identifiable application or executable parent after ordinary shells
- **THEN** the native record contains historical launch-host evidence
- **AND** the radar exposes only name, kind and observation time, independently of input ownership, liveness, pane binding and action permissions

#### Scenario: Shared display across surfaces
- **WHEN** a native record has valid launch-host evidence
- **THEN** terminal, menu bar, phone, iPad and Web retain the harness identity and may show the host name
- **AND** conversation details show the observation time and retain current input-client uncertainty

#### Scenario: Missing or ambiguous evidence
- **WHEN** no safe launch host can be observed or an old session has no observation
- **THEN** an unconfirmed native client is labeled as an external session
- **AND** no process-wide scan or guessed historical backfill establishes a source

#### Scenario: Later hooks retain historical context
- **WHEN** a non-identity hook updates the session
- **THEN** the last launch-host observation and its timestamp remain historical
- **AND** a new identity hook replaces it, including clearing it when the new observation is unknown
