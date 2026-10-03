# remote-access (delta)

## MODIFIED Requirements

### Requirement: A Mac can move between Direct servers without re-pairing its devices

The system SHALL let a Mac move to another Direct server (`gtmux tunnel --server <id>`, or
the menu bar), authenticated by the device's own account, keeping its account and its port.
The move SHALL be a plain reassignment: the Mac reconnects on the new server once its
account is there, with no window to wait out and no two tunnels running at once.

After a successful move, the CLI and menu bar SHALL mark the server named by this Mac's
persisted Direct dial URL as current. A provisioner listing that still reports the previous
assignment SHALL NOT make the selected route jump back. If the local URL is absent or is
not among the offered servers, the listing MAY use the provisioner's current server.
The menu bar SHALL not start a second route-list refresh while a move is in progress.

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

After a move succeeds, the current server in the CLI, menu bar, and owner route list SHALL
be resolved from this Mac's persisted dial URL among the offered servers. A stale
provisioner `current` value SHALL NOT replace a known local route. When the local URL is
absent or not offered, the provisioner value MAY be used. The menu bar SHALL NOT refresh
the route list concurrently with a move.

#### Scenario: The provisioner still lists the old route after a move

- **WHEN** a move has persisted a new Direct dial URL on the Mac but the next server list
  names the former server as current
- **THEN** the CLI, menu bar and owner route list mark the locally selected server as
  current, and a menu-bar refresh cannot race the move and restore the former selection

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

#### Scenario: Stale list right after a move

- **WHEN** the Mac has saved the destination URL but the provisioner still lists the old
  server as current
- **THEN** the destination remains marked current after the menu bar reloads the list

#### Scenario: Refresh requested while moving

- **WHEN** a refresh is requested while a menu-bar route move is in progress
- **THEN** the refresh waits for the move's own post-completion reload
