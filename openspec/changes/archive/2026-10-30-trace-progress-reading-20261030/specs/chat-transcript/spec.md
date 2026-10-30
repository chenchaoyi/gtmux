## ADDED Requirements

### Requirement: Codex structured and freeform calls
The shared Codex reader SHALL preserve ordered structured arguments and freeform input, including custom exec code, with existing UTF-8 limits and partial-line handling. Outputs, reasoning and mirrored item events SHALL remain excluded.

#### Scenario: Read the updated surface
- **WHEN** a supported source or browser surface is opened
- **THEN** it follows the shared presentation and evidence rules without changing input authorization
