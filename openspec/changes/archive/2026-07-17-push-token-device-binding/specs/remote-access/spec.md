# remote-access — delta

## ADDED Requirements

### Requirement: Revoke unregisters the device's push token; the token store is master-only

`POST /api/devices/revoke` SHALL, after removing a roster entry, unregister every push
token bound to that device id (so revoking a device stops its notifications in one step,
consistent with the scoped-revoke authorization already in place). The push-token
management endpoints SHALL be MASTER-only:

- `GET /api/push/tokens` SHALL return the registered tokens REDACTED (prefix only) to the
  master token, and refuse any non-master caller (`403`).
- `POST /api/push/forget {deviceId|orphans|all}` SHALL drop the selected tokens for the
  master token, and refuse any non-master caller (`403`).

`POST /api/push/register` / `POST /api/push/unregister` (a device managing its OWN token)
remain available to an authenticated device as before.

#### Scenario: Revoke drops the token

- **WHEN** the master (or an owner device, per the scoped-revoke rules) revokes a device
  that had a registered push token
- **THEN** the revoke also unregisters that device's push token

#### Scenario: Token store is master-only

- **WHEN** a device or guest token calls `GET /api/push/tokens` or `POST /api/push/forget`
- **THEN** it is refused (`403`), and only the master token (the Mac's own CLI) may
  inspect or clear the store
