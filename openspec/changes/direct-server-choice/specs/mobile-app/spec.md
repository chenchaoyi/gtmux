# mobile-app (delta)

## ADDED Requirements

### Requirement: A paired Mac may answer at more than one address

The app SHALL fetch, on its first connection to a paired Mac and on every later one, the
addresses that Mac says it could answer at, and SHALL keep them with that Mac. When the
address it is using stops answering, it SHALL try the others, bounded the same way every
pairing step is, and SHALL save the one that answered as the address it uses from then on.

Only an address the Mac itself reported SHALL enter the list. A Mac is reported unreachable
only after every address in its list has been tried.

#### Scenario: The Mac moved to another server

- **WHEN** the saved address stops answering and another address the Mac reported does
- **THEN** the app connects through that one, saves it, and the user sees no more than the
  reconnect the app already shows

#### Scenario: None of the addresses answer

- **WHEN** every address for that Mac fails
- **THEN** the app reports it unreachable, with the same plain diagnosis it gives today,
  and keeps the addresses for the next attempt

#### Scenario: Addresses come only from the Mac

- **WHEN** the app refreshes a Mac's addresses
- **THEN** it stores what that Mac reported on an authenticated call and nothing else

### Requirement: The app shows which Direct server carries the Mac

The app SHALL show, with the connection state, WHERE that connection goes: the server's
name as a place, in the reader's language, when the Mac reports one. When there is nothing
to name (a local address, the standard tunnel) it SHALL show the address instead, as it
did before. While the Mac is unreachable it SHALL still name where it was last reached.

The app SHALL NOT offer to change servers. Moving cuts the connection the request would
travel through, so the result could not be observed from here; and the case that would want
it — the server is down while the user is away — is the case where the phone cannot reach
the Mac to ask at all. That choice lives on the Mac.

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
