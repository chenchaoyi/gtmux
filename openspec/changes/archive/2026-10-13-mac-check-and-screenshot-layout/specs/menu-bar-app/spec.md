## ADDED Requirements
### Requirement: Screenshot editor scales with its window
The editor SHALL prefer the capture's original point size for initial placement,
clamp its window to the current display's usable frame without an arbitrary 85%
limit, and fit its image and annotations proportionally to the live canvas when
the user resizes the window. Exports SHALL keep the original pixel resolution.

#### Scenario: Resize a Retina capture
- **WHEN** the user enlarges or shrinks the editor containing annotations
- **THEN** the image and marks change size together with the same aspect ratio and image coordinates
- **AND** the exported image retains its original pixel size

### Requirement: Screenshot notes support multiline text
Return SHALL insert a newline in the note. Command-Return SHALL request a send
only when sending is available and no input-method composition is in progress.
The note area SHALL have a stable height and scroll long text, so editing a note
does not move the capture. Existing send guards SHALL continue to apply.

#### Scenario: Enter a multiline note
- **WHEN** a user presses Return while writing a note
- **THEN** a newline is inserted without sending
- **AND** Command-Return invokes the same guarded action as the Send button
