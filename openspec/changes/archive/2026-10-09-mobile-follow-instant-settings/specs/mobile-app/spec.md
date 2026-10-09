## MODIFIED Requirements

### Requirement: Owner desktop follow form
Paired phone and iPad owners SHALL use a shared safe-area-aware, bounded per-conversation settings form. Each toggle SHALL save immediately using the confirmed read revision and prevent duplicate writes and dismissal during a pending write. Server or conversation changes SHALL isolate late responses. Guests and incompatible cores SHALL NOT expose an actionable follow control.

#### Scenario: Conflict
- **WHEN** another device has updated the same policy
- **THEN** the form reads and displays current settings without claiming the rejected write succeeded; if that read fails, further changes require a successful reload

#### Scenario: Late response
- **WHEN** the user switches server while a request is pending
- **THEN** the old response cannot update another conversation

### Requirement: Clear desktop follow sheet
Phone and iPad SHALL share one desktop-conversation settings sheet with one HQ-follow switch and a subordinate group of notification/knowledge switches. Each option SHALL have a short explanation and an accessible switch label. Switch appearance SHALL match native App settings, without a grey enabled-track override. The sheet SHALL show scope once, retain a scrolling body, provide Done in the header and reserve fixed space for loading, update and error feedback. Save/Cancel draft actions SHALL NOT be shown. All three rows SHALL remain present throughout reading, toggling and error states. Before the first successful read, placeholders SHALL avoid claiming default permissions.

#### Scenario: Optional capabilities
- **WHEN** HQ follow is disabled
- **THEN** the optional switches remain in place but are unavailable; enabling HQ grants neither option automatically, and disabling HQ clears both grants while retaining existing records

#### Scenario: Save error
- **WHEN** a write fails, conflicts or loses its receipt
- **THEN** the form reads canonical settings before allowing another change, explains the result without claiming success, and offers Reload; a failed read disables all changes

## ADDED Requirements

### Requirement: Follow toggles apply immediately without shifting the sheet
The shared phone/iPad follow sheet SHALL issue one revision-checked write per toggle using the last confirmed policy. It SHALL keep the same option rows and status area through loading, saving and failure. HQ is a prerequisite for the two independent optional capabilities. Done SHALL close the sheet except while a write and its reconciliation are pending. Unmounting SHALL prevent late responses from refreshing a different server or conversation.

#### Scenario: Master toggle
- **WHEN** an owner turns HQ follow off and then on
- **THEN** both child grants are cleared by the off write and are not restored by the on write, and no row is inserted or removed

#### Scenario: Failed update
- **WHEN** an update is not acknowledged
- **THEN** the displayed values are reconciled with a fresh read, or edits remain disabled until an explicit reload succeeds; permissions are not retried automatically

