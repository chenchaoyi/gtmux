## ADDED Requirements

### Requirement: Screenshot to an agent from the menu bar

The menu bar app SHALL start a screenshot from a global hotkey (⌥⌘4) and from a button in
the popover's header row, without changing the existing ⌥⌘G palette hotkey. Before
capturing it SHALL check Screen Recording permission; without it, it SHALL request it once
and explain where to grant it, with a button that opens that System Settings pane, and
SHALL NOT capture. It SHALL close its own popover and palette before capturing, capture a
region through the system's interactive capture, and treat a capture cancelled with Esc as
nothing having happened. The annotation editor SHALL open only after the capture, on the
screen under the pointer, with arrow, rectangle and text tools, undo and redo, and Esc to
cancel. Copy, Save and Send SHALL export the same flattened image at the capture's full
pixel resolution. Copy SHALL put PNG and TIFF on the pasteboard; Save SHALL write a PNG to a
location the user chooses. Send SHALL go to an agent pane chosen in the editor, defaulting
to the agent pane a terminal showed most recently, then the last target used, then the most
recently active agent, and SHALL happen only when the user presses Send.

#### Scenario: Copy a marked region

- **WHEN** the user presses ⌥⌘4, selects a region, draws a rectangle and presses Copy
- **THEN** the pasteboard holds the region with the rectangle, at the capture's pixel size
- **AND** no pane received anything

#### Scenario: Cancel the capture

- **WHEN** the user presses Esc during the selection
- **THEN** no editor opens, no file is left behind and no pane is touched

#### Scenario: No Screen Recording permission

- **WHEN** the app lacks Screen Recording permission and the user presses ⌥⌘4
- **THEN** nothing is captured and the app explains how to grant it, with a button that
  opens the System Settings pane

#### Scenario: The editor keeps its work on a second hotkey press

- **WHEN** an editor is open and the user presses ⌥⌘4 again
- **THEN** the open editor comes to the front instead of a new capture starting

### Requirement: Sending a screenshot never answers a prompt or sends twice

Before sending, the editor SHALL re-read the target pane. It SHALL refuse with a message,
sending nothing, when the pane is gone or the agent is **waiting** on the user, since typed
text and Enter could answer a permission prompt or a question. It SHALL send through
`gtmux send --json <pane> --message-file - --attach <png>` and report the result: delivered
or queued as success; a refused draft, a duplicate or an unconfirmed delivery with the
reason, keeping the editor and its image open. It SHALL NOT retry by itself, and its retry
control SHALL tell the user to check the pane first.

#### Scenario: The agent is waiting on a permission prompt

- **WHEN** the target pane's status is waiting and the user presses Send
- **THEN** nothing is typed into the pane and the editor says the agent is waiting

#### Scenario: The input box has someone's unsent text

- **WHEN** `send` refuses because the pane's input box holds a draft
- **THEN** the editor stays open and says so, and nothing was appended to the draft

#### Scenario: The pane closed while the editor was open

- **WHEN** the target pane no longer exists at Send
- **THEN** the editor says the pane is gone and offers the remaining panes
