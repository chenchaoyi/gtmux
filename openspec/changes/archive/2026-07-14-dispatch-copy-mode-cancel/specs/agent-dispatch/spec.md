# agent-dispatch Specification

## ADDED Requirements

### Requirement: Delivery drops the pane out of copy-mode first

Before delivering task text to a pane, the system SHALL drop the pane out of any tmux
mode (copy-mode / view-mode). While a pane is in a mode, `paste-buffer` and `Enter`
are interpreted as mode-navigation and never reach the program, so an un-cancelled
delivery is silently swallowed (and can be mis-verified as landed). The system SHALL
exit the mode (`send-keys -X cancel`) before pasting, and SHALL treat exiting as a
no-op when the pane is not in a mode. This applies to the verified delivery path
(`gtmux spawn` and `gtmux send`) AND to the plain write paths (`gtmux send`
`--no-verify`/`--no-enter`/`--key` and `POST /api/send`). Land-verification is
otherwise unchanged.

#### Scenario: A scrolled pane still receives the dispatch

- **WHEN** `gtmux send`/`spawn` delivers to a pane that is in copy/view-mode
- **THEN** the pane is dropped out of the mode before the text is pasted
- **AND** the payload lands in the input box and is verified as landed, not swallowed

#### Scenario: A non-scrolled pane is not disturbed

- **WHEN** delivery targets a pane that is NOT in a mode
- **THEN** no `-X cancel` is sent (it would error "not in a mode")
- **AND** delivery proceeds exactly as before

#### Scenario: The API and plain send paths also exit the mode

- **WHEN** `POST /api/send` or `gtmux send --no-verify`/`--key` writes to a pane in
  copy/view-mode
- **THEN** the pane is dropped out of the mode before the key/text is sent, so the
  input reaches the agent
