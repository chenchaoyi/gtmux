## ADDED Requirements

### Requirement: The Mac has one HQ workspace window
The Mac SHALL expose situation board, knowledge, machine and usage as named sidebar destinations in the existing HQ workspace. Menu-card shortcuts and empty-state links SHALL select that destination in the same retained window. The legacy segmented HQ reader window SHALL NOT remain reachable. Knowledge SHALL retain search, list/detail navigation, judgment actions, archive export, backup restore and selective migration. Resource destinations SHALL NOT show an HQ instruction composer or accept a send. Switching away from a conversation SHALL preserve its draft.

#### Scenario: Open resources from the menu card
- **WHEN** the user opens knowledge, board, machine or usage from the menu card
- **THEN** the existing workspace is brought forward with that destination selected
- **AND** reopening HQ without a destination preserves the current selection

#### Scenario: Return to a draft
- **WHEN** the user switches from an unsent matter/global/new-task draft to a resource destination and back
- **THEN** the original draft is retained and no instruction has been sent

### Requirement: Resource reads follow workspace visibility
The Mac SHALL reuse bounded, gated resource reads and refresh only the selected resource destination. Closing or hiding the workspace, or leaving resources, SHALL stop its periodic resource reads. A late read from a prior destination SHALL NOT replace the selected destination's contents. Failures SHALL retain the last valid snapshot and refresh remains available.

#### Scenario: Knowledge followed by usage
- **WHEN** the user switches from knowledge to usage
- **THEN** usage is read without continuing knowledge polling
- **WHEN** the window closes
- **THEN** no resource timer remains active

### Requirement: Matter headers identify conversations without repeating replies
Matter headers SHALL show linked conversation names, agent, current pane/location and status when an exact current identity is available. Saved pane locators SHALL be labelled historical when no current identity matches, and SHALL NOT inherit another occupant's status. The header SHALL NOT repeat the HQ assessment above the full conversation. Existing stored assessments and source records SHALL remain intact.

#### Scenario: A linked worker exits and its pane is reused
- **WHEN** no current candidate matches the full linked conversation identity
- **THEN** its saved pane is shown as a last-known pane and its source remains readable
- **AND** another conversation occupying that pane does not supply the status or title
