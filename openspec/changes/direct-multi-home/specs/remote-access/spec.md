# remote-access (delta)

## ADDED Requirements

### Requirement: A Mac is reachable on every Direct server it may use

A Mac SHALL hold a tunnel on each Direct server it is entitled to, not one, so every one of
those addresses reaches it at the same time. Its reverse port is unique across the fleet, so
the same path names that Mac on every server.

Each (device, server) pair SHALL have its OWN credential, bound to that device's one port.
A server's account file SHALL carry only the credentials minted for it, so a server that is
compromised yields nothing that works anywhere else.

A route that cannot be dialled SHALL NOT stop the others: the Mac keeps whatever routes it
has, and a client uses whichever answers.

One route SHALL be the PREFERRED one: what the pairing code carries and what a client uses
until it has a reason to use another. Setting it SHALL NOT disconnect anything, because
nothing moves.

#### Scenario: Two routes, both live

- **WHEN** a Mac is entitled to two Direct servers
- **THEN** both addresses reach it at the same time, and a client may use either

#### Scenario: A credential does not travel

- **WHEN** the credential minted for a device on one server is presented to another
- **THEN** that server refuses it, even though the same device is entitled to both

#### Scenario: One route is down

- **WHEN** one server stops answering
- **THEN** the Mac keeps its other routes, clients keep working through them, and nothing
  the Mac does is required

#### Scenario: Setting the preferred route

- **WHEN** the user sets another route as preferred
- **THEN** the pairing code carries that address from then on, and no device loses its
  connection, because the Mac did not move
