## ADDED Requirements

### Requirement: Conventional interface terminology

Built-in Mac, phone, iPad and web interfaces SHALL use concise setting names, explicit action labels and factual status descriptions. Explanations SHALL describe the control's effect without personification or rhetorical questions. Equivalent controls SHALL use consistent English and Chinese terminology. Copy changes SHALL preserve user content, protocol fields, compatibility keys and historical release notes.

#### Scenario: Creating a share link

- **WHEN** a user names a share link and selects its permissions
- **THEN** the field identifies the link name and the explanation identifies selected panes and view/input permissions, without implying verified recipient identity

#### Scenario: No diagnostic problems are recorded

- **WHEN** the diagnostic view has no warning or error records
- **THEN** it reports the absence of records rather than claiming that the system has never had a problem
