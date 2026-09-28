# menu-bar-app (delta)

## ADDED Requirements

### Requirement: Remote access choices look and read the same in both Mac windows

Preferences and Pair your phone SHALL present the shared Remote access setting with
the same Access and Route labels, option order, and segmented controls. Route SHALL
appear only when Anywhere is selected. Direct server choices in Pair your phone
SHALL appear beneath Route in the same access card.

#### Scenario: Compare both windows while Anywhere is active

- **WHEN** the user opens Preferences and Pair your phone while Anywhere is active
- **THEN** both windows show Access (Off / Local network / Anywhere) followed by
  Route (Standard / Direct), and the pairing window shows Direct servers below Route

#### Scenario: Local access needs no route

- **WHEN** the user selects Off or Local network in either window
- **THEN** both windows hide Route while showing the same Access choices
