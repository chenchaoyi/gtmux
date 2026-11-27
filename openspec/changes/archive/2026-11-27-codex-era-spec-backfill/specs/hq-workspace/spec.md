## ADDED Requirements

### Requirement: Record confirmed repair work before closing HQ diagnoses

Before marking an HQ diagnosis reviewed, HQ SHALL verify remaining in-scope work, original user authorization, and current ownership. When a confirmed, authorized defect has no verified owner, HQ SHALL register it in the same turn as a separate open matter with a stable ID and record its `goal-followup` using the original `authority_ref`, while preserving the parent matter's external hold, user pauses/refusals, and dispatch gates.

#### Scenario: Diagnosis uncovers an unclaimed authorized defect while an external hold remains
- **WHEN** HQ reviews a diagnosis that confirms a separate authorized defect with no verified owner while the parent matter waits on an external condition
- **THEN** HQ registers the defect via `register` with a stable ID citing the diagnosis and original matter, reads back the registration, and records its next step via `goal-followup` with the original `authority_ref` before closing the diagnosis (stating why in the assessment when a diagnosis leaves no work)
- **AND** the parent matter keeps its external hold, recovery condition, scheduled check time, and any covering user pause or refusal unchanged

#### Scenario: Repeat evidence arrives for an already registered repair matter
- **WHEN** subsequent evidence references the same defect
- **THEN** HQ reuses the existing repair matter and verifies its current owner before any dispatch instead of creating a duplicate matter
