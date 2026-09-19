# menu-bar-app (delta)

## ADDED Requirements

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
