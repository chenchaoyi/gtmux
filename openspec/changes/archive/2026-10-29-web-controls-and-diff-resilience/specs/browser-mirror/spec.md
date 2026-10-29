## MODIFIED Requirements

### Requirement: Desktop "workbench" mode

On a wide screen the served UI SHALL offer a "workbench" mode: a left session/agent
rail plus a freeform board of draggable, resizable pane tiles (multiple panes visible
at once), with layout presets, snap-to-grid and a ⌘K command palette. Polling SHALL NOT open new panels automatically, including when an old saved preference enabled it. Layout changes affect
only this browser; input is available in panes authorized by `/api/share`. It uses
the same authenticated `/api/pane` + `/api/transcript` data as the single-pane mirror.

#### Scenario: Arrange multiple panes

- **WHEN** the user is on a wide screen and adds panes to the workbench board
- **THEN** each pane renders live and can be dragged/resized/arranged, with the
  arrangement applied via presets or manual placement; authorized panes may accept input

### Requirement: Session tools preserve the presentation mode

Chat and Terminal SHALL be presentation modes. Trace and Diff SHALL be available
as direct actions beside, but visually separate from, the mode switch in panels and focus. Modes and actions SHALL stay below session identity across viewport sizes. Font size SHALL have one entry in Settings. A tool SHALL retain the
underlying mode, draft and reading position, and retire pending reads on close or
identity/access changes. Readable external sessions SHALL offer Trace without
Terminal, Diff or input. Focused Chat SHALL retain the normal conversation layout.

#### Scenario: Inspect changes from focused Chat

- **WHEN** a reader opens Diff from focused Chat and closes it
- **THEN** the original conversation, selected mode and unsent draft remain
- **AND** Trace is available directly beside the presentation switch

### Requirement: Workspace reading hierarchy

The actual supervisor role SHALL have a distinct pinned section above ordinary
session status groups. Its avatar SHALL use the canonical three-cell gtmux brand mark and an independent status badge. Empty workspaces SHALL offer a usable session-picker action
and a secondary context-specific, attributed excerpt. Native select controls SHALL
retain 44px height, inset arrows and visible focus. Tool summaries SHALL be compact
and unboxed; records without additional input SHALL NOT open empty inspectors.

#### Scenario: Begin from an empty workspace

- **WHEN** the board has no open panels
- **THEN** a clear session-selection action opens the existing searchable picker
- **AND** the atmospheric quotation remains secondary to that action

