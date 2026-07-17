# mobile-app — delta

## ADDED Requirements

### Requirement: An owner-only screen manages this Mac's sharing

The app SHALL offer a "Manage this Mac" screen, reachable ONLY on an owner
connection (a paired device — `!isGuest`); a guest connection SHALL NOT surface
its entry. The screen SHALL let the owner manage SHARING for the connected Mac,
mirroring the menu bar: toggle the consent switch, see each share link with its
per-link scope, edit a link's See/Type per session, create a new link (name +
per-session scope in one step), and revoke a link. It SHALL also show the paired
DEVICE roster READ-ONLY, with a one-line note that revoking a device and changing
the remote-access door are done on the Mac (decision B). The screen SHALL NOT
present controls for the withheld actions, so no button 403s.

#### Scenario: An owner opens the management screen

- **WHEN** the app is connected with a device (owner) token
- **THEN** the "Manage this Mac" entry is available, and it shows the share
  controls (consent, per-link See/Type, create, revoke a link) plus a read-only
  device roster

#### Scenario: A guest never sees management

- **WHEN** the app is connected via a share link (guest)
- **THEN** the "Manage this Mac" entry is absent, and the app never calls the
  management endpoints

#### Scenario: The owner edits a link's scope from the phone

- **WHEN** the owner toggles a session's Type on a link and confirms
- **THEN** the app calls `POST /api/share/set` for that link only, and the change
  is reflected (per-link, not global)
