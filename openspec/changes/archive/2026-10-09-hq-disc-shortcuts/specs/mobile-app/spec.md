## ADDED Requirements

### Requirement: Floating HQ resource shortcuts

The owner floating HQ disc SHALL offer Usage and Knowledge base on long press when HQ is available. A short tap SHALL retain its normal HQ navigation; movement beyond the drag threshold SHALL cancel the hold and SHALL NOT open a page, even if the finger returns to its origin. Interruption and unmount SHALL cancel pending timers. A recognized hold SHALL provide haptic feedback and suppress its release tap. VoiceOver SHALL provide equivalent named actions. Destination navigation SHALL wait for native menu dismissal on iOS, carry the current HQ identity and requested sheet through compact and demo navigation, and reuse the existing current-Mac resource sheets. Initial resource reads SHALL distinguish loading and failure from confirmed empty data. Guests SHALL NOT receive the floating HQ entrance; absent HQ SHALL retain its start explanation.

#### Scenario: Hold then choose

- **WHEN** an owner holds the disc without dragging and chooses Knowledge base
- **THEN** the menu closes before that sheet opens on the HQ page, and releasing the hold does not also open the default HQ page

#### Scenario: Drag or cancel

- **WHEN** the gesture moves beyond the threshold or is interrupted before recognition
- **THEN** no shortcut menu or navigation occurs and no pending timer survives unmount
