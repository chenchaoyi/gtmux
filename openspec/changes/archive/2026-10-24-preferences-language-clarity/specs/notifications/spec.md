## ADDED Requirements

### Requirement: Notification settings hierarchy

The menu-bar preferences SHALL group the desktop notification switch with its explanation, system permission state and test controls. Test feedback SHALL remain attached to the test action and distinguish system acceptance from visible delivery. Terminal-tab markers SHALL have their own setting group. Setting titles SHALL be concise names rather than conversational instructions.

#### Scenario: Reviewing notification settings

- **WHEN** the user opens notification preferences
- **THEN** the desktop notification control, permission status and test action are grouped, while the terminal marker is separate

#### Scenario: Testing notification delivery

- **WHEN** macOS accepts a test notification
- **THEN** feedback beside the test action reports submission without claiming that a banner appeared
