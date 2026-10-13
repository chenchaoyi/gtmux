## ADDED Requirements
### Requirement: Mac check details do not repeat the summary
A Mac environment row SHALL show its current result once. Expanded details SHALL
retain distinct technical guidance, without repeating the same label, purpose or
result. Long results SHALL remain readable in full when expanded.

#### Scenario: Installed prerequisite expanded
- **WHEN** a short installed result has separate technical guidance
- **THEN** the installed result appears once and the distinct guidance remains readable

#### Scenario: Long or legacy result
- **WHEN** an older report lacks a description or a long result is expanded
- **THEN** the available guidance and full result remain accessible without duplicate text
