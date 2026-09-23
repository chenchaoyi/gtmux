# remote-access (delta)

## MODIFIED Requirements

### Requirement: Paid "Direct" tier via redeemable code

The system SHALL provide a paid "Direct" tier layered on the self-hosted (443)
backend: `gtmux tunnel --redeem <code>` exchanges a purchased code, together with the
device's id, at the tunnel provisioner (`POST /direct/redeem`, validated against a
`DIRECT_CODES` store) for a Direct server URL, a chisel account belonging to THAT
DEVICE alone, and the reverse port assigned to it. The redeem MAY carry a server id or a
region preference, and the answer SHALL name the server the device was assigned. They are
persisted to the local self-tunnel config so subsequent `gtmux tunnel --backend self` runs
need no manual `GTMUX_SELFTUNNEL_URL`/`SECRET`. The in-process Chisel client (no external
binary) carries the transport.

No Direct credential SHALL be shared between devices. Each account SHALL be allowed to
bind exactly its own reverse port and nothing else — no other device's port, and no
forward tunnel to the server's own services. A device's port SHALL be unique across the
whole fleet, not merely within one server, so it stays the same wherever the device is
assigned. A code SHALL mint accounts for at most a configured number of devices;
re-redeeming on a device that already has an account SHALL return that account. Revoking a
code SHALL remove the accounts it minted, so the devices using them can no longer connect.

A Direct server SHALL accept only these accounts: no shared or catch-all user. It SHALL
obtain them from the provisioner through an endpoint that authenticates THAT server, and
SHALL receive only the accounts assigned to it, so no server holds the credentials of
devices that do not use it. A failure to obtain them SHALL keep the accounts it already
has rather than replace them with none. It SHALL NEVER run with zero accounts, since the
transport turns authentication off when it has none; a server-local account that can bind
nothing SHALL always be present. When an account is removed, the server SHALL end every
established session, so the removed device cannot keep serving through a tunnel it opened
before.

#### Scenario: Redeem a Direct code

- **WHEN** the user runs `gtmux tunnel --redeem <code>` with a valid code
- **THEN** the provisioner returns a Direct URL, this device's own account and its
  port; they are written to the self-tunnel config, and later `--backend self` runs
  connect with no manual env

#### Scenario: Redeem into a chosen region

- **WHEN** the redeem carries a region preference and a server in that region accepts new
  devices
- **THEN** the device is assigned that server, and the answer says which one, so the Mac
  can show it

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

- **WHEN** a Direct server has no device accounts (fresh, or the last one revoked)
- **THEN** it still refuses every client that does not hold an account

#### Scenario: One server never holds another's accounts

- **WHEN** a Direct server fetches its account file
- **THEN** it receives the accounts assigned to it, and the account of a device assigned
  elsewhere is absent

#### Scenario: A refused account is explained, not retried in silence

- **WHEN** `gtmux tunnel --backend self` starts with an account the server no longer
  accepts (the code was revoked, or the credentials were replaced)
- **THEN** it stops and tells the user to redeem their code again, instead of retrying the
  authentication failure indefinitely

## ADDED Requirements

### Requirement: Direct servers are configuration, discovered at run time

The set of Direct servers SHALL be data held by the provisioner, not code: one record per
server carrying its id, base URL, region, a label in both languages, whether it accepts new
devices, and whether it is offered to every code or only to named ones. Adding, editing or
disabling a server SHALL require no change to the provisioner's source, no client release
and no app release.

No client SHALL carry a built-in list of servers. The CLI and the menu bar SHALL ask the
provisioner (`GET /direct/servers`) and render what it returns, so a server added today is
selectable by installations that already exist. A provisioner with no list configured SHALL
behave as one implicit server at its single configured address, so a deployment that
predates this requirement needs no migration.

Every Direct server SHALL answer a liveness path that no user's Mac sits behind, so a
client can measure the round trip to a server it has never used and an operator can check a
server with no device paired to it.

#### Scenario: A server added without a release

- **WHEN** the operator adds a server record to the provisioner's store and nothing else
- **THEN** an already-installed Mac lists it, can redeem onto it and can move to it, with
  no new version of the CLI, the menu-bar app or the phone app

#### Scenario: A server not offered to everyone

- **WHEN** a server is marked as offered only to named codes
- **THEN** a redeem that does not carry one of those codes is never assigned to it, and a
  listing for such a caller does not present it as a choice

#### Scenario: Measuring a server before using it

- **WHEN** a Mac lists the servers
- **THEN** it can time each one's liveness path, including servers it has no account on,
  and show the round trip beside each

#### Scenario: A provisioner with no list

- **WHEN** no server list is configured
- **THEN** redeem, listing and the account file behave exactly as a single server at the
  configured address

### Requirement: A Mac can move between Direct servers without re-pairing its devices

The system SHALL let a Mac move to another Direct server (`gtmux tunnel --server <id>`, or
the menu bar), authenticated by the device's own account, keeping its account and its port.
The move SHALL be a plain reassignment: the Mac reconnects on the new server once its
account is there, with no window to wait out and no two tunnels running at once.

A Mac SHALL tell an authenticated client every address it could answer at — its port on
each server it may use, the current one first. A paired client SHALL fetch that list when it
connects, keep it, and, when its saved address stops answering, try the others before
reporting the Mac unreachable, saving the one that worked. Probing another server SHALL be
safe by construction: a device's port is unique across the fleet, so no other device can be
behind that path on any server.

The pairing code SHALL keep carrying a single address, since its size is bounded by what a
scannable QR holds; the list is delivered over the connection that pairing establishes.

The product SHALL state what a move costs rather than leave it to be discovered: a device
that paired but never connected afterwards knows only the address it scanned, and a guest
share link minted before a move stops working and has to be re-minted.

#### Scenario: A phone that was away while its Mac moved

- **WHEN** a paired phone that has connected at least once opens after its Mac moved to
  another server
- **THEN** its saved address fails, it tries the other addresses it was given, finds the
  Mac, and saves that address, with nothing for the user to do

#### Scenario: A phone that never connected after pairing

- **WHEN** a phone paired and was closed before it ever connected, and the Mac then moved
- **THEN** it knows only the address from its pairing code, reports the Mac unreachable,
  and the Mac's menu bar offers a pairing code to scan again

#### Scenario: A probe that finds no Mac

- **WHEN** a client tries its Mac's port on a server the Mac is not on
- **THEN** nothing answers for that path, and no credential reaches another device, because
  that port belongs to that device on every server

#### Scenario: A guest link minted before the move

- **WHEN** a Mac moves and a guest link minted before the move is opened
- **THEN** the link does not reach the Mac, and the user was told at the moment of moving
  that existing links have to be re-minted

#### Scenario: The move is authenticated

- **WHEN** a move is requested without the device's own account credentials
- **THEN** the provisioner refuses it, and no device is reassigned
