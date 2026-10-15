## ADDED Requirements

### Requirement: Menu survives app-initiated focus changes
The menu SHALL stay visible when Restore opens terminal windows, so its existing progress remains readable. Automatically shown upgrade notes SHALL open alongside the just-opened menu, not replace it. Ordinary menu presentations SHALL retain transient behavior. A protected presentation SHALL still close on an outside click, Escape in the menu, status-item toggle or another explicit navigation action. Closing or completing an operation SHALL NOT reopen a dismissed menu or activate it over the terminal. Protection and event monitors SHALL end when the menu closes.

#### Scenario: Restore activates a terminal
- **WHEN** Restore starts from an open menu and terminal activation changes focus
- **THEN** the menu remains visible with the existing restore progress
- **AND** an explicit dismissal stays dismissed even when Restore completes

#### Scenario: Automatic notes become ready
- **WHEN** upgrade notes become ready while the menu is open
- **THEN** the reader and menu remain visible together
- **AND** interacting with the reader does not dismiss the menu
- **AND** a reply arriving after menu dismissal waits until the next opening
