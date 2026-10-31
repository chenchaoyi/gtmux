## ADDED Requirements

### Requirement: Session-led Trace details and HQ briefing cards
The App SHALL lead Trace with the selected session name, show full recorded tool input with wrapping, and avoid repeating an individual input in its group heading. HQ SHALL show actual board, knowledge and usage summaries before independent matter cards. Summary requests SHALL preserve owner scope, cancellation and transient-error semantics.

#### Scenario: A long command is inspected on a phone
- **WHEN** a reader selects an individual tool in an expanded call group
- **THEN** details show its full recorded input with wrapping and exact copy

#### Scenario: HQ opens with available summaries
- **WHEN** an owner opens the matter overview
- **THEN** resource summaries precede matters without requiring another reader
- **AND** cards show authoritative current states and dated updates without treating delivery as completion

## MODIFIED Requirements

### Requirement: Mobile HQ uses the shared matter overview
The App SHALL offer Needs input, Following and recent changes from the authoritative HQ workspace read projection. It SHALL stack groups in narrow panes and flow current-matter cards in a shared grid when the available content width and text scale permit. A matter reader SHALL show explicit assessment, participants and paged saved updates; receipt delivery MUST NOT imply completion. Talk to HQ, resources, drafts, composer and session tools SHALL retain the existing console behavior.

#### Scenario: A matter is opened without sending an instruction
- **WHEN** an owner opens a matter and chooses Ask HQ
- **THEN** the existing HQ console opens with an editable prefill and sends nothing automatically

#### Scenario: A historical participant's pane was reused
- **WHEN** the original agent/home/session no longer matches the live roster or its membership ended
- **THEN** the participant remains readable metadata and cannot open the new occupant of that pane

#### Scenario: An old or temporarily unreachable server
- **WHEN** the workspace list returns 404 or 501
- **THEN** the App retains the legacy HQ page
- **WHEN** refreshing fails transiently
- **THEN** the last overview remains with a retry action
- **WHEN** access is revoked
- **THEN** private overview and matter content are cleared
