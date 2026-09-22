# remote-access (delta)

## MODIFIED Requirements

### Requirement: Paid "Direct" tier via redeemable code

The system SHALL provide a paid "Direct" tier layered on the self-hosted (443)
backend: `gtmux tunnel --redeem <code>` exchanges a purchased code, together with the
device's id, at the tunnel provisioner (`POST /direct/redeem`, validated against a
`DIRECT_CODES` store) for the Direct server URL, a chisel account belonging to THAT
DEVICE alone, and the reverse port assigned to it. They are persisted to the local
self-tunnel config so subsequent `gtmux tunnel --backend self` runs need no manual
`GTMUX_SELFTUNNEL_URL`/`SECRET`. The in-process Chisel client (no external binary)
carries the transport.

No Direct credential SHALL be shared between devices. Each account SHALL be allowed to
bind exactly its own reverse port and nothing else — no other device's port, and no
forward tunnel to the server's own services. A code SHALL mint accounts for at most a
configured number of devices; re-redeeming on a device that already has an account SHALL
return that account. Revoking a code SHALL remove the accounts it minted, so the devices
using them can no longer connect.

The Direct server SHALL accept only these accounts: no shared or catch-all user. It SHALL
obtain them from the provisioner through an authenticated endpoint, and a failure to
obtain them SHALL keep the accounts it already has rather than replace them with none. It
SHALL NEVER run with zero accounts, since the transport turns authentication off when it
has none; a server-local account that can bind nothing SHALL always be present. When an
account is removed, the server SHALL end every established session, so the removed
device cannot keep serving through a tunnel it opened before.

#### Scenario: Redeem a Direct code

- **WHEN** the user runs `gtmux tunnel --redeem <code>` with a valid code
- **THEN** the provisioner returns the Direct URL, this device's own account and its
  port; they are written to the self-tunnel config, and later `--backend self` runs
  connect with no manual env

#### Scenario: Invalid or spent code

- **WHEN** the redeemed code is unknown/expired
- **THEN** the command reports it clearly and writes no config (no opaque failure)

#### Scenario: One device cannot take another's port

- **WHEN** a device's account tries to bind a reverse port assigned to a different
  device, whether or not that device is connected
- **THEN** the server refuses it

#### Scenario: A code used on too many devices

- **WHEN** a code has already minted accounts for the configured number of devices and a
  new device redeems it
- **THEN** the redeem is refused with a message saying the code is in use on that many
  devices, and no account is minted

#### Scenario: A revoked device that is online

- **WHEN** a code is revoked while a device it unlocked is connected
- **THEN** that device's sessions end within one sync, and its reconnection is refused

#### Scenario: A server with no device accounts

- **WHEN** the Direct server has no device accounts (fresh, or the last one revoked)
- **THEN** it still refuses every client that does not hold an account

#### Scenario: A refused account is explained, not retried in silence

- **WHEN** `gtmux tunnel --backend self` starts with an account the server no longer
  accepts (the code was revoked, or the credentials were replaced)
- **THEN** it stops and tells the user to redeem their code again, instead of retrying the
  authentication failure indefinitely
