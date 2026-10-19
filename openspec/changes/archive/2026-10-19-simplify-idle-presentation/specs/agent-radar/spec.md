## ADDED Requirements

### Requirement: Quiet agents share one user-facing idle group
Menu bar, phone, iPad and Web SHALL group neutral `running` agents with confirmed `idle` agents under Idle / 空闲, with matching summary counts. They SHALL NOT add a separate standby group or rewrite core status, completion evidence or timestamps. Neutral rows SHALL retain their neutral dot rather than a completed checkmark. Existing error, watched and external-conversation categories SHALL remain separate.

#### Scenario: Neutral DeepSeek session
- **WHEN** a managed DeepSeek pane has neutral `running` status without a confirmed completion
- **THEN** it appears once in the existing idle group with a neutral badge
- **AND** its underlying status and unknown completion timestamp remain unchanged

#### Scenario: Mixed quiet sessions
- **WHEN** confirmed idle and neutral managed sessions coexist
- **THEN** the idle group count includes both, excluding failures, HQ, watched and external conversations according to their existing categories
- **AND** accessible status labels use the existing idle vocabulary
