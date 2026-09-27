# agent-integration Specification Delta

## ADDED Requirements

### Requirement: Agent capability declarations are wired to implementations

The registry conformance checks SHALL verify that every declared transcript capability has a
registered parser, every hook-equipped agent has a hook installer and display mapping, and
every agent declaring dedicated event semantics has a classifier table. A missing capability
implementation SHALL fail the check with the agent key and capability named.

#### Scenario: A declared transcript parser is missing

- **WHEN** an agent manifest declares transcript support without a parser
- **THEN** the conformance check fails and names that agent and transcript capability

### Requirement: The Codex transcript parser declares its observed event shapes

The Codex Tier 2 transcript parser SHALL have sanitized fixtures for the event shapes it
claims to support. Unknown event records SHALL be skipped without turning injected context
into a user prompt or preventing later recognized turns from being read.

#### Scenario: A transcript contains an unknown event

- **WHEN** the Codex parser reads an unknown event between recognized records
- **THEN** it ignores that record and continues parsing subsequent recognized turns
