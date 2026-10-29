## ADDED Requirements

### Requirement: Session tools preserve the presentation mode

Chat and Terminal SHALL be presentation modes. Trace and Diff SHALL be available
from a separate session-tools entry in panels and focus. A tool SHALL retain the
underlying mode, draft and reading position, and retire pending reads on close or
identity/access changes. Readable external sessions SHALL offer Trace without
Terminal, Diff or input. Focused Chat SHALL retain the normal conversation layout.

#### Scenario: Inspect changes from focused Chat

- **WHEN** a reader opens Diff from focused Chat and closes it
- **THEN** the original conversation, selected mode and unsent draft remain
- **AND** Trace is available from the same tools entry

### Requirement: Workspace reading hierarchy

The actual supervisor role SHALL have a distinct pinned section above ordinary
session status groups. Empty workspaces SHALL offer a usable session-picker action
and a secondary context-specific, attributed excerpt. Native select controls SHALL
retain 44px height, inset arrows and visible focus. Tool summaries SHALL be compact
and unboxed; records without additional input SHALL NOT open empty inspectors.

#### Scenario: Begin from an empty workspace

- **WHEN** the board has no open panels
- **THEN** a clear session-selection action opens the existing searchable picker
- **AND** the atmospheric quotation remains secondary to that action

### Requirement: Authenticated conversation attachment preview

Browser conversation links SHALL reuse the App document reference and metadata
validation contract and owner-only artifact endpoints. An explicit click SHALL load
bounded, digest-verified bytes. Images SHALL have bounded dimensions before decoding.
Closing or losing access SHALL retire reads and release private object URLs. Raw
HTML and arbitrary external resources SHALL NOT execute through the preview.

#### Scenario: Reject corrupted image bytes

- **WHEN** image content differs from its verified metadata digest or size
- **THEN** the preview shows a recoverable failure without decoding that content
