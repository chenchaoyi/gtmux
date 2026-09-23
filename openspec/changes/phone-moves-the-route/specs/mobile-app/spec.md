# mobile-app (delta)

## ADDED Requirements

### Requirement: The phone can change which route its Mac uses

The app SHALL show, for a paired Mac, the Direct routes it may use, each with the round trip
THIS DEVICE measured against that route, and SHALL let the user move the Mac to another one.
A route that does not answer SHALL say so rather than show a time.

Before moving, the app SHALL say what it costs: every other paired device drops for the
seconds it takes and comes back by itself, a device that paired but never connected has to
scan again, and guest links minted before the move stop working.

After moving, the app SHALL find the Mac on its new route through the addresses it already
keeps, and SHALL say which route it ended up on.

For a GUEST connection the app SHALL NOT show this at all.

#### Scenario: A slow route, from where the user is

- **WHEN** the user opens the routes for their paired Mac
- **THEN** each route shows the round trip this phone measured, and the one in use is marked

#### Scenario: Moving from the phone

- **WHEN** the user picks another route and confirms
- **THEN** the Mac moves, this phone follows it to the new route on its own, and says which
  one it is on

#### Scenario: A guest connection

- **WHEN** the connection is a guest link
- **THEN** no route section appears
