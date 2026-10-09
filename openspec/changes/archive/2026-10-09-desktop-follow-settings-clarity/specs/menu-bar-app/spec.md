## ADDED Requirements

### Requirement: Compact desktop follow form
Desktop conversation settings SHALL expose one HQ-follow switch, with notification and knowledge switches grouped below it only when enabled. Identity/scope and capability explanations SHALL each appear once. Editable settings SHALL appear only after a successful initial read, and the fixed footer SHALL offer Cancel and a revision-checked save.

#### Scenario: Initial read fails
- **WHEN** loading conversation settings fails
- **THEN** the form offers reload without presenting default-valued editable settings

#### Scenario: Stop then re-enable before saving
- **WHEN** HQ follow is turned off and back on in the draft
- **THEN** both optional permissions remain off until individually selected
