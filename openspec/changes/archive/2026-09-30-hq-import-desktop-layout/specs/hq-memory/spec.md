## ADDED Requirements

### Requirement: Import sheets follow the current task
Archive selection and preview SHALL use compact layouts. Detailed migration review SHALL
use a larger bounded workspace. The current step and primary action SHALL be clear;
actions SHALL be grouped in a footer. Password input SHALL appear only after selection
of an encrypted archive, identified from its header rather than file extension. UI header
inspection SHALL be bounded and SHALL NOT replace backend archive validation.

#### Scenario: Selecting an unencrypted backup
- **WHEN** an owner selects a plain archive, even with an age filename suffix
- **THEN** no password field appears and preview still validates the entire archive

#### Scenario: Reviewing knowledge
- **WHEN** selected content is ready for review
- **THEN** the sheet provides a bounded list/detail workspace with review gates and apply actions visible
