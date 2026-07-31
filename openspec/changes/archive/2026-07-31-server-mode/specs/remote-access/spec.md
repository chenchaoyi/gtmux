# remote-access — delta

## ADDED Requirements

### Requirement: Server mode is remotely visible and remotely revocable, never remotely enablable

The serve API SHALL expose the server-mode state to OWNER-scoped clients only
(`GET /api/servermode`, the same document `gtmux server-mode status --json` returns), and
SHALL accept an OWNER-scoped request that turns server mode OFF. A request to turn server
mode ON SHALL be refused by the server with an explicit reason, regardless of client,
because enabling requires a local interactive administrator authorization and an
unattended machine has nobody to answer it. GUEST tokens SHALL NOT be able to read or
change server mode at all — it is a machine-level control, not a per-pane one. A change of
server-mode state SHALL be pushed to connected clients over the existing live-update
channel so a remote surface does not show a stale state.

#### Scenario: Phone turns server mode off

- **WHEN** a paired owner device posts a request to turn server mode off
- **THEN** the server restores sleep, reports `state:"off"` with
  `last_exit.reason:"revoked"`, and connected clients see the change without polling

#### Scenario: Remote enable is refused

- **WHEN** any client — owner device included — posts a request to turn server mode on
- **THEN** the server refuses with a reason stating that server mode can only be enabled at
  the Mac, and no system setting changes

#### Scenario: A guest sees nothing

- **WHEN** a guest share token requests the server-mode endpoint
- **THEN** the request is denied like any other owner-only endpoint, and no server-mode
  state is disclosed
