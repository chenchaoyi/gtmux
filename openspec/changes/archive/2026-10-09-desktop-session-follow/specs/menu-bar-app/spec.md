## ADDED Requirements

### Requirement: Desktop conversation settings
The menu bar SHALL show desktop rows in a separate group with the saved policy badge and a per-conversation settings window. Draft changes SHALL NOT alter the saved badge until a successful receipt; failure/conflict SHALL preserve draft choices and offer reload.

#### Scenario: Save
- **WHEN** the user enables HQ follow
- **THEN** notification and knowledge remain off unless independently selected

#### Scenario: Stop
- **WHEN** the user chooses status-only and saves
- **THEN** the primary action stops follow and clears both permissions
