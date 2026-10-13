## ADDED Requirements

### Requirement: Readable environment check results
The structured doctor report SHALL describe the purpose of known checks in the
selected language while preserving their status, value and technical note.
The menu-bar report SHALL give rows identities unique across sections, present
each row only under its own heading, separate purpose from technical details,
and use the shared system-font design tokens. Check time SHALL follow the app's
selected language. Older reports without purpose descriptions SHALL remain readable.

#### Scenario: Multiple groups share row offsets
- **WHEN** installation and HQ groups both have rows numbered from zero
- **THEN** their display identities differ and each group shows only its own checks

#### Scenario: A technical check needs explanation
- **WHEN** a row checks history-limit or an agent hook
- **THEN** its normal display explains the checked capability
- **AND** the full technical result and note remain accessible in expandable details

#### Scenario: Report from an older CLI
- **WHEN** a valid report has no purpose descriptions
- **THEN** existing notes are readable without rejecting the report
