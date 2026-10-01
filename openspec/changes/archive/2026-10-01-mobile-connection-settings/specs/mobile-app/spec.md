## MODIFIED Requirements

### Requirement: The app shows which Direct server carries the Mac

The app SHALL show, with the connection state, WHERE that connection goes: the server's
name as a place, in the reader's language, when the Mac reports one. When there is nothing
to name (a local address, the standard tunnel) it SHALL show the address instead, as it
did before. While the Mac is unreachable it SHALL still name where it was last reached.

For every paired owner, Settings SHALL retain a Route entry regardless of loading,
read failure or the number of available Direct routes. It SHALL show the current or
last reported place and open the route page while connected. A guest SHALL not
see or change routes; their Status row SHALL still show the destination. Offline
owners SHALL see their last known place and a connection requirement.

The route page SHALL distinguish loading, failed reads and valid empty lists,
offer retry, and never describe zero routes as one route. A failed refresh SHALL
retain known choices and disable moving until a successful read. A new Mac SHALL
not inherit the previous Mac's choices. Choices SHALL appear before phone-side
probes finish; only reachable alternatives are selectable. Older reads and probes
SHALL NOT overwrite newer requests or an accepted move. Settings SHALL refresh
on return and reconnection. If no route is marked current, it SHALL show a checking
state rather than a blank value. Sharing & pairing SHALL retain its owner-only
share links and device roster.

#### Scenario: A failed read does not remove a setting

- **WHEN** an owner's route request fails
- **THEN** Settings still shows Route and the saved place, and the route page shows
  failure and retry instead of claiming that the Mac has one route

#### Scenario: A new Mac replaces an earlier request

- **WHEN** the user switches Macs before a route read or probe finishes
- **THEN** the previous Mac's response cannot populate the new Mac's route choices

#### Scenario: A Mac on a named server

- **WHEN** the Mac reports the server it is on
- **THEN** the connection line reads as the state and that place ("Connected · Shanghai"),
  in the reader's language

#### Scenario: Nothing to name

- **WHEN** the Mac is reached over a local address or the standard tunnel
- **THEN** the connection line keeps showing the address, as before

#### Scenario: Unreachable

- **WHEN** the Mac cannot be reached
- **THEN** the line says so and still names where it was last reached

#### Scenario: Returning from a move

- **WHEN** the owner moves to another route and returns from the route page
- **THEN** Settings fetches the current route again and names the selected place

#### Scenario: One route never answers

- **WHEN** the Mac reports multiple routes and one phone-side probe times out
- **THEN** Settings shows the route row and the route page shows the places from
  the returned list immediately, while measured times fill in independently
