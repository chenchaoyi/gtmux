## ADDED Requirements

### Requirement: Start a session on the paired Mac

The app SHALL offer New session in the radar and All panes for owner connections, with a labelled empty-radar action. Offline controls SHALL be disabled; guests and demo SHALL have no creation controls. A keyboard-ready form SHALL identify the active Mac, accept an optional name, preview canonicalization and offer Create and open. Compact canvases SHALL use a bottom sheet; regular canvases SHALL use a bounded centred form with the same behavior.

#### Scenario: Create and open on phone or iPad
- **WHEN** the owner creates a session and receives its real pane identity
- **THEN** the form closes before Workspace opens that pane in Terminal, using phone navigation or the iPad main area

#### Scenario: Busy form prevents a second creation
- **WHEN** creation is in flight
- **THEN** submission, name editing and dismissal are disabled and the creating state is visible

#### Scenario: Uncertain result remains actionable
- **WHEN** a network interruption or creation failure prevents confirmation
- **THEN** the form retains the same request ID and name, offers explicit Retry and Check sessions, and does not assume success

#### Scenario: Recoverable conflict or unsupported server
- **WHEN** creation reports a duplicate name or unsupported endpoint
- **THEN** the app preserves the form and offers a new name or an instruction to update gtmux on the Mac

#### Scenario: Server changes during creation
- **WHEN** a form unmounts while its request is in flight
- **THEN** its late result does not open a pane in the newly selected Mac's workspace

