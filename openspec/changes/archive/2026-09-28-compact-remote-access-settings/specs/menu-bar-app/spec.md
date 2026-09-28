# menu-bar-app (delta)

## MODIFIED Requirements

### Requirement: Remote access choices look and read the same in both Mac windows

Preferences and Pair your phone SHALL present Access, Connection method, and
Route in that order. The first two SHALL use full-width, left-aligned choice
controls with the same labels and option order in both windows. Connection
method SHALL appear only for Anywhere, and Route only for Direct. The Route
list SHALL share the choice controls' width and identify the selected server on
its row. Optional method guidance
SHALL be available from a clickable help control; Preferences SHALL keep the
current address available there rather than as a permanent settings row.
Errors from failed changes SHALL remain visible beside the controls. A move
in either window SHALL update the selected route and address in the other.
The Route measurement footer SHALL name latency and keep its relative age
current while either window remains open; a hover hint SHALL explain that
round-trip latency is measured from this Mac.

#### Scenario: Compare the two Mac windows

- **WHEN** Anywhere and Direct are active in both windows
- **THEN** Access, Connection method, and Route share a left edge, the choice
  controls and Route list fill the same width, and the selected server is marked

#### Scenario: Find details without reading them on every visit

- **WHEN** the user opens help for a choice in Preferences
- **THEN** its explanation is available; Access help also shows the current
  address when one exists, while the pairing card continues to show that address

#### Scenario: A setting fails to change

- **WHEN** the CLI refuses an access or route change
- **THEN** the error remains visible in the window where the change was requested

#### Scenario: Read a recent route measurement

- **WHEN** the Direct server list has just been measured
- **THEN** both windows say that latency was just measured and offer another
  measurement; the measurement origin is available on hover
