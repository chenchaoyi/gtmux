# remote-access — delta

## ADDED Requirements

### Requirement: Owner devices manage sharing; the device roster and door stay Mac-scoped

The system SHALL authorize the remote-management surface by caller scope so that an
owner device (a paired phone / browser / terminal — `full` scope, like the master
token) can MANAGE SHARING remotely, while the device roster and the remote-access
door remain restricted:

- The SHARE-management endpoints — `POST /api/share/config` (consent + the global
  lists), `POST /api/share/new`, `POST /api/share/set` — SHALL accept any `full`
  caller (master OR owner device) and SHALL refuse a `guest` (`403`).
- `GET /api/devices` SHALL be restricted to `full` callers (master or owner
  device) and SHALL refuse a `guest` (`403`).
- `POST /api/devices/revoke` SHALL honor the caller's scope: a master MAY revoke
  ANY roster entry; an owner device MAY revoke ONLY a `guest` entry (a share
  link) and SHALL be refused (`403`) when the target is a paired device; a guest
  SHALL be refused entirely.
- Toggling the remote-access door (starting/stopping serve or the tunnel) SHALL
  remain a local Mac operation with no remote endpoint.
- `GET /api/share/link?id=<id>` SHALL re-hand an existing guest link's TOKEN to a
  `full` caller (master or owner device) and SHALL refuse a `guest` (`403`) and an
  unknown id (`404`), so a host who didn't copy a link at mint time can copy it
  again (the CLI `gtmux share link <id>` and the menu-bar row's copy button both use
  it; the token rides the `#t=` URL fragment, never a bare field).

`GET /api/share` (the caller's own capability) SHALL remain available to any
authenticated scope, unchanged.

#### Scenario: An owner device manages a share link

- **WHEN** an owner device token calls `POST /api/share/new`, `POST /api/share/set`,
  or `POST /api/share/config`
- **THEN** the operation is authorized (as it would be for the master token)

#### Scenario: A guest cannot reach the management surface

- **WHEN** a `guest` token calls `GET /api/devices`, `POST /api/share/config`,
  `POST /api/share/new`, or `POST /api/share/set`
- **THEN** each is refused (`403`) — closing the prior unguarded device-list/revoke

#### Scenario: An owner may revoke a share link but not a paired device

- **WHEN** an owner device calls `POST /api/devices/revoke` for a `guest` entry
- **THEN** the share link is revoked
- **WHEN** the same owner device calls it for a paired (`device`) entry
- **THEN** it is refused (`403`), and only the master token (from the Mac) can
  revoke a paired device

#### Scenario: Re-hand an existing link's URL

- **WHEN** a `full` caller (master or owner device) calls `GET /api/share/link?id=<id>`
  for a live guest link
- **THEN** it returns that link's token (for rebuilding the `#t=` URL)
- **WHEN** a `guest` calls it, or the id is unknown
- **THEN** it is refused (`403`) or reported not-found (`404`) respectively
