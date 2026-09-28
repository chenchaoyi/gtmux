# menu-bar-app (delta)

## MODIFIED Requirements

### Requirement: Remote access choices look and read the same in both Mac windows

Preferences and Pair your phone SHALL present the shared Remote access setting
with the same Access and Connection method labels, option order, and segmented
controls. Connection method SHALL appear only when Anywhere is selected. Both
windows SHALL show the same Route list of named Direct servers beneath Connection
method when Direct is selected. Its heading SHALL name the current server and
its selected row SHALL say it is in use. A successful move SHALL update the
selected row in both open windows before fresh measurements return.

#### Scenario: Compare both windows while Direct is active

- **WHEN** the user opens Preferences and Pair your phone while Anywhere and Direct are active
- **THEN** both windows show Access (Off / Local network / Anywhere), Connection
  method (Standard / Direct), and the same named server Route list

#### Scenario: Move to another Direct server

- **WHEN** the user confirms a move in either window and the CLI accepts it
- **THEN** both windows mark the new server as current immediately and refresh
  their measurements and reachable address

#### Scenario: Local access needs no connection method or route

- **WHEN** the user selects Off or Local network in either window
- **THEN** both windows hide Connection method and Route
