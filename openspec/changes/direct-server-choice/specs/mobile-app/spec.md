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
