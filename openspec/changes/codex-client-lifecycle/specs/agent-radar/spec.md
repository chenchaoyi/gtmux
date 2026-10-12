## ADDED Requirements

### Requirement: Present Codex current-control evidence consistently
The core SHALL provide compatible current-control evidence and capabilities to CLI,
menubar, web, iPhone and iPad. Creation source SHALL NOT be presented as confirmed
current controller. Pure history SHALL NOT increase live counts. Conflicting physical
panes SHALL remain accessible in the pane browser without silently selecting an owner.

#### Scenario: Current control is unknown
- **WHEN** a stored thread has no verified input route
- **THEN** surfaces SHALL present unconfirmed control and only verified capabilities
- **AND** they SHALL NOT fabricate a completion transition or notification

#### Scenario: Two actual attachments conflict
- **WHEN** two observed attachments for one thread have conflicting control evidence
- **THEN** surfaces SHALL expose the conflict and retain exact physical locators
- **AND** they SHALL NOT deduplicate by title or redirect input silently
