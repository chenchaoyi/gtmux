## ADDED Requirements

### Requirement: Inspect the participant without changing conversation state

The system SHALL open a participant profile from a message avatar, retaining
reading position, draft and disclosure state. Human profiles SHALL use the agreed
human identity and Dune quotation. Agent profiles SHALL describe the identified
participant and distinguish historical message facts from current runtime facts.

#### Scenario: Historical speaker

- **WHEN** a reader opens an older message or another agent's sender avatar
- **THEN** no facts from the current pane, account or installed binary are borrowed

### Requirement: Collect scoped native profile facts

The system SHALL share profile data through the Go core, use per-agent native
contracts, keep subscription, task mode, permission, variant and reasoning facts
distinct, and omit unsupported fields. Missing facts SHALL remain unrecorded.
Reading SHALL NOT run model turns or execute user configuration. Sensitive process
arguments SHALL be omitted before persistence; profile access SHALL require owner
scope and a verified conversation binding.

#### Scenario: Guest or ambiguous session

- **WHEN** a guest requests a profile, or its conversation binding is ambiguous
- **THEN** private profile facts and process evidence are not returned

### Requirement: Fold each reply at its own content boundary

The system SHALL support independent per-turn reply folding with an attached
copy/share/fold footer, a two-line collapsed preview and full-text copy/share.
Global reply folding and tool-card disclosure SHALL remain separate controls.

#### Scenario: Fold in globally expanded mode

- **WHEN** a reader collapses one reply while other replies are expanded
- **THEN** only that reply folds and its avatar remains independently actionable
