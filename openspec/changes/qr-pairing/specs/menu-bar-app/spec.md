## ADDED Requirements

### Requirement: Allow phone access with a pairing QR

The system SHALL provide an "Allow phone access" control that surfaces (or starts)
`gtmux serve` and renders a pairing QR encoding `{v:1,url,token,name}` for a
reachable address + the serve token, so the phone can pair by scanning instead of
typing. The token is a secret: it SHALL be shown only on explicit user action and
never logged.

#### Scenario: Show the pairing QR

- **WHEN** the user opens "Allow phone access" with `gtmux serve` reachable
- **THEN** the popover shows a QR encoding the reachable url + serve token + a Mac
  name, plus the human-readable host(s)

#### Scenario: Serve not running

- **WHEN** no `gtmux serve` is running
- **THEN** the control offers to start one (or explains how) before showing the QR

#### Scenario: Restraint

- **WHEN** the pairing panel is shown
- **THEN** the token is not printed to logs and the panel notes it grants
  read-only remote access over the local network/VPN
