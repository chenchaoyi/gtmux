## ADDED Requirements

### Requirement: Screenshot delivery feedback is prompt and translucent
After a verified successful receipt the screenshot editor SHALL close promptly
without a one-second success dwell. Sending SHALL keep the captured image visible
under a blurred, translucent backdrop with legible foreground feedback, honoring
the system's reduced-transparency setting. Failed or uncertain outcomes SHALL keep
their existing recovery behavior and SHALL NOT be reported as delivered.

#### Scenario: A successful receipt belongs to the current editor
- **WHEN** screenshot delivery confirms success
- **THEN** the current editor closes within the short feedback interval and a late
  completion cannot close a newer editor

#### Scenario: Delivery is unconfirmed
- **WHEN** the result is uncertain or failed
- **THEN** the editor retains the image, note and recovery controls
