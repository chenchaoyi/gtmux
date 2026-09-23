# mobile-app (delta)

## ADDED Requirements

### Requirement: The device chooses which route it uses

The app SHALL let the user pick which of its Mac's routes this device uses, and SHALL show
what each one costs FROM THIS DEVICE: the round trip it measures itself, not the Mac's.
A route that does not answer SHALL say so rather than show a time.

Switching SHALL be local: it changes the address this app uses, asks nothing of the Mac, and
leaves every other device on the fleet where it was. It SHALL take effect on the next
request, with no re-pairing.

When the route in use stops answering, the app SHALL fall back to another that does, as it
already does for a Mac that moved, and SHALL say which route it ended up on.

#### Scenario: A slow route on this device

- **WHEN** the user opens the route list on the phone
- **THEN** each route shows the round trip measured from the phone, and picking another one
  takes effect immediately for this phone alone

#### Scenario: Nobody else is affected

- **WHEN** this phone switches route
- **THEN** the Mac keeps every route it has, and another paired device keeps using the one
  it was using

#### Scenario: The chosen route stops answering

- **WHEN** the route this device chose goes quiet
- **THEN** the app falls back to one that answers and says which route it is on
