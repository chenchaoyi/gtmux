## ADDED Requirements

### Requirement: Ambiguous bare knowledge links are reported

The knowledge linter SHALL resolve exact entry IDs before bare slugs. If a bare
slug matches multiple live entries, it SHALL report `ambiguous-link` and SHALL
NOT assign the reference to an arbitrary entry.

#### Scenario: Two topics share one slug

- **WHEN** `pitfalls/shared` and `corrections/shared` are live and a body links to `[[shared]]`
- **THEN** lint reports an ambiguous link, while `[[pitfalls/shared]]` resolves exactly
