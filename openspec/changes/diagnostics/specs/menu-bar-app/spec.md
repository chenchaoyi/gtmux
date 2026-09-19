# menu-bar-app (delta)

## ADDED Requirements

### Requirement: The pairing window explains an unreachable address from tunnel status

When the "Pair your phone" window cannot reach its own pairing address, it SHALL explain
why from `status/tunnel.json`, for either backend: connected and fresh means this Mac
cannot see its own address but the tunnel is up and a phone on cellular connects; down
means no device connects, shown with the recorded error; stale or missing means it cannot
tell and SHALL say only that it cannot reach the address yet. It SHALL NOT read
cloudflared's log to decide.

#### Scenario: Direct on a network that hijacks DNS

- **WHEN** the backend is Direct, the window's probe fails, and `status/tunnel.json`
  reports `down` with a resolver error
- **THEN** the window says no device can connect and shows that error, and does not tell
  the user that a phone on cellular connects

#### Scenario: Standard, visible to the phone but not to the Mac

- **WHEN** the backend is Standard, the window's probe fails because this network maps the
  address to a private IP, and `status/tunnel.json` reports `connected` and is fresh
- **THEN** the window says this network blocks the address and a phone on cellular connects

### Requirement: The menu bar app writes to the gtmux log store

The menu bar app SHALL write its entries to the gtmux log store in the shared schema and
under the same redaction: actions it starts (from the menu or a window) with actor
`menubar`, notifications it posts, and failures (a CLI call, a pairing-code mint, an
update) with their HTTP status or error, plus reachability verdicts. It SHALL mirror them
to the unified log under subsystem `com.gtmux.menubar`. `GTMUXBAR_DEBUG` SHALL add debug
entries.

#### Scenario: A pairing code could not be minted

- **WHEN** the menu bar fails to mint a pairing code
- **THEN** the store gains a `mint.failed` entry from component `menubar` with the HTTP
  status or error, and `gtmux logs --component menubar` shows it
