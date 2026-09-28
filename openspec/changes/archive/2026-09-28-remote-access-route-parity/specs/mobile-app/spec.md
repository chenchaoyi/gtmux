# mobile-app (delta)

## MODIFIED Requirements

### Requirement: The app shows which Direct server carries the Mac

The paired app SHALL name the current Direct route in Settings and on the route
page when the Mac reports multiple available routes. The current route SHALL be
visibly marked in the list and exposed to accessibility. After a move is accepted,
the route page SHALL mark the destination immediately while it reconnects;
Settings SHALL refresh the route list when the user returns. If no route is marked
current, Settings SHALL show an explicit checking state rather than a blank value.
If the Mac cannot return choices, Status SHALL still show a saved route name;
Settings SHALL refresh choices when the connection returns.

#### Scenario: A route change is accepted

- **WHEN** the paired owner moves the Mac to another route and returns to Settings
- **THEN** the route page marks the selected destination and Settings fetches the
  current route again

#### Scenario: Current marker is missing

- **WHEN** multiple routes exist but none is marked current
- **THEN** the route value says it is checking instead of appearing empty
