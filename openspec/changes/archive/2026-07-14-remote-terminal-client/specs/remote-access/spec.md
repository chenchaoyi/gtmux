## ADDED Requirements

### Requirement: WebSocket attach endpoint

The serve contract SHALL include `GET /api/attach?id=%N` — a WebSocket endpoint that
bridges a tmux pane's PTY to the caller. It SHALL be authenticated and scope-gated: an
owner (master/device token) may attach any pane; a `guest` token may attach ONLY a
view-allowed pane, and the server SHALL refuse the upgrade otherwise. The bridge SHALL
use binary frames with a one-byte opcode (client→server `INPUT`/`RESIZE`/`PAUSE`/
`RESUME`, server→client `OUTPUT` carrying raw PTY bytes), DROP write frames
(`INPUT`/`RESIZE`) for a pane the caller may not type into, and bound its buffering with
client-driven `PAUSE`/`RESUME` flow control. Scope enforcement is server-side and
authoritative; a client flag never widens it.

#### Scenario: Guest upgrade refused for a non-viewable pane

- **WHEN** a guest opens `/api/attach?id=%N` for a pane not on its view allowlist
- **THEN** the server refuses the WebSocket upgrade and spawns no PTY

#### Scenario: View-only input is dropped

- **WHEN** a guest attached to a view-only pane sends an `INPUT` frame
- **THEN** the server does not write it to the pane

### Requirement: The CLI is a first-class client of the serve contract

The gtmux CLI (`gtmux attach`) SHALL be a first-class client of the `gtmux serve`
contract, alongside the web page and the mobile app, using the SAME token-scope model:
a device/master token attaches as an owner (full), a `guest` token attaches
scope-restricted (view-only panes are read-only). The server enforces scope identically
regardless of client surface.

#### Scenario: A terminal client with a guest token is restricted

- **WHEN** `gtmux attach https://host/#t=<token> %N` connects with a guest token
- **THEN** it is restricted exactly as a guest browser/app — refused a non-viewable pane, read-only on a view-only one
