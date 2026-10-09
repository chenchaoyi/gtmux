## ADDED Requirements

### Requirement: Clear desktop follow sheet
Phone and iPad SHALL share one desktop-conversation settings sheet with one HQ-follow switch and a subordinate group of notification/knowledge switches. Each option SHALL have a short explanation and an accessible switch label. The sheet SHALL show scope once, retain a scrolling body and fixed save/cancel footer, and distinguish loading from saving.

#### Scenario: Optional capabilities
- **WHEN** the user enables HQ follow
- **THEN** the two optional switches appear without being granted automatically

#### Scenario: Save error
- **WHEN** saving fails or the read revision conflicts
- **THEN** the draft is retained, saved state is not claimed and reload is offered
