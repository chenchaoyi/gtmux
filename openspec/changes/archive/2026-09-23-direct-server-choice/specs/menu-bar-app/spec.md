# menu-bar-app (delta)

## ADDED Requirements

### Requirement: The Direct section chooses a server

The Direct part of the remote-access sheet SHALL list the servers the provisioner offers
this Mac, each with its region label in the reader's language and the measured round trip
from this Mac, and SHALL mark the one in use. The list SHALL NOT print an address: opening
a server SHALL show that server's own detail, where the address belongs.

A server's detail SHALL name the region, its round trip and the address this Mac has (or
would have) there, and SHALL carry the one action that fits it: moving this Mac there, or,
for the server in use, handing over the pairing code. Moving SHALL first say what it costs,
including that guest links minted before it stop working.

The list SHALL come from the provisioner at run time, never from a list built into the app,
so a server added by the operator appears without an app update.

#### Scenario: Opening a server that is not in use

- **WHEN** the user opens a server this Mac is not on
- **THEN** its detail shows the region, the round trip and the address this Mac would have
  there, and offers to move this Mac to it

#### Scenario: Opening the server in use

- **WHEN** the user opens the server this Mac is on
- **THEN** its detail shows that Mac's current address there and hands over the pairing code

#### Scenario: A server added by the operator

- **WHEN** the operator adds a server to the provisioner's list
- **THEN** it appears in the sheet with its label and round trip, with no app update

#### Scenario: Moving says what it costs first

- **WHEN** the user chooses to move to another server
- **THEN** they are told that devices which have connected before follow on their own, that
  a device which never connected after pairing has to scan again, and that guest links
  minted before the move have to be re-minted
