## ADDED Requirements

### Requirement: Remote access confirmation uses one factual explanation

Preferences, Pair your phone and pairing's access setup SHALL use the same
confirmation before enabling Anywhere. It SHALL name paired devices and share
link recipients as possible users and explain that access stays enabled after a
restart until turned off. The confirmation SHALL not imply that the address
alone grants access or that only the Mac's master token can authorize a client.

#### Scenario: Enable Anywhere from any Mac entry point

- **WHEN** the user selects Anywhere in Preferences or pairing setup
- **THEN** the same confirmation appears before the setting changes, and Cancel
  leaves the setting unchanged

### Requirement: Knowledge action wording matches the phone

The Mac knowledge reader SHALL describe `retire` as marking an entry no longer
applicable, and SHALL explain where `land`, `carry` and `withdraw` record their
results without metaphorical instructions.

#### Scenario: Retire an entry

- **WHEN** the user opens the `retire` action
- **THEN** the action says "标记为不再适用" in Chinese while retaining the
  existing `retire` ledger operation
