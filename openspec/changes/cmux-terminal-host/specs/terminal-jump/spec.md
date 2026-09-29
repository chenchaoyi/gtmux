## ADDED Requirements

### Requirement: cmux host sessions use their own terminal driver

The system SHALL recognize a cmux-hosted session before the embedded Ghostty
identity using `CMUX_WORKSPACE_ID` inside the terminal or a `cmux.app` tmux-client
ancestor. It SHALL use cmux's scriptable terminal titles and focus command for
exact jumps and frontmost viewing checks. Restore/new SHALL create one cmux
workspace per tmux session through AppleScript, including when invoked outside
cmux, and SHALL report failures.

#### Scenario: Focus a cmux-hosted tmux session

- **WHEN** a tmux client belongs to cmux and its terminal title names the session
- **THEN** `gtmux focus` focuses that cmux terminal panel, including titles with
  an alert decoration, without activating Ghostty

#### Scenario: Restore into cmux

- **WHEN** `gtmux restore` needs to reopen detached sessions in cmux
- **THEN** it opens a workspace per session, each running a quoted tmux attach
  command, and returns a failure if the cmux CLI cannot create a workspace

#### Scenario: Unavailable cmux panel

- **WHEN** cmux cannot return its currently focused terminal title
- **THEN** `IsViewing` returns false rather than suppressing a notification
