# remote-access (delta)

## ADDED Requirements

### Requirement: An owner device may move its Mac to another route remotely

The system SHALL let a PAIRED (owner) device ask its Mac which Direct routes it may use and
move it to one of them, through the Mac's own API. A GUEST token SHALL be refused on both,
and a guest surface SHALL NOT present the operation at all.

The move performed remotely SHALL be the same operation the Mac performs locally: the same
reassignment at the provisioner, the same reconnect, the same result. An unknown route SHALL
be refused without changing anything.

#### Scenario: The owner's phone moves the Mac

- **WHEN** a paired device asks its Mac to move to a route it may use
- **THEN** the Mac moves exactly as it would from its own menu bar, and answers with the
  route it is on

#### Scenario: A guest may not

- **WHEN** a guest token asks for the routes, or asks to move the Mac
- **THEN** both are refused, and nothing changes

#### Scenario: A route that is not there

- **WHEN** the requested route is not one this Mac may use
- **THEN** the request is refused and the Mac stays where it is
