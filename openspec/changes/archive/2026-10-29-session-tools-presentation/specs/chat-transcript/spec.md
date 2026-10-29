## ADDED Requirements

### Requirement: Shared attachment envelope normalization

Observed complete local-image prompt envelopes SHALL become Markdown image links
in the shared public transcript. Code examples, malformed envelopes and external
paths SHALL remain literal. Original provider logs SHALL remain unchanged.

#### Scenario: Read the same image prompt on App and Web

- **WHEN** a prompt contains an observed local image envelope outside a code block
- **THEN** every transcript consumer receives the same escaped Markdown destination
- **AND** file resolution still verifies that destination against its conversation
