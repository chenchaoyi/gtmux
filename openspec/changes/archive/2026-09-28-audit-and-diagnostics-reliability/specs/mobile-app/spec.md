## ADDED Requirements

### Requirement: Mobile controls speak actions rather than test identifiers

Interactive controls SHALL expose a localized, meaningful accessibility label.
Automation identifiers SHALL remain in `testID` and SHALL NOT become the label a
screen reader speaks. Small helper text on the connection and pairing surfaces
SHALL remain legible in both light and dark appearance.

#### Scenario: A screen reader reaches the composer

- **WHEN** VoiceOver focuses the send, attachment, or expand control
- **THEN** it hears the control's action in the selected language, not a `testID`
