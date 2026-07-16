# remote-terminal-client — delta

## ADDED Requirements

### Requirement: Attach pairs a terminal as an owner surface

`gtmux attach` SHALL support the owner-pairing medium: an attach target whose
fragment carries an enroll code (`#c=<code>`) SHALL be redeemed once via
`POST /api/enroll` (device name = the local hostname) for an OWNER device token,
persisted locally (`~/.config/gtmux/remotes.json`, mode 0600, keyed by host), and
the attach proceeds with `full` scope. A later bare `gtmux attach <host>` SHALL
reuse the persisted token for that host before requiring `--token`. Guest share
links (`#t=`) SHALL keep their existing behavior. Revoking the device on the host
(`gtmux pair revoke`) SHALL invalidate the persisted token immediately (the next
request fails auth).

#### Scenario: Pair a terminal with the one-liner

- **WHEN** the user runs the printed `gtmux attach 'https://host/#c=<code>'` on
  another computer
- **THEN** the code is redeemed for an owner device token, stored in
  remotes.json (0600), and the session attaches with full scope

#### Scenario: Subsequent attach needs no credential

- **WHEN** the same user later runs `gtmux attach host` with no flags
- **THEN** the persisted token authenticates and the attach proceeds as owner

#### Scenario: Host-side revocation cuts the terminal off

- **WHEN** the owner revokes that terminal's device on the host
- **THEN** the persisted token stops authenticating (`401`) on its next use
