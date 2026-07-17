# push-notifications — delta

## ADDED Requirements

### Requirement: Push tokens are bound to the enrolled device

Each registered APNs push token SHALL carry the roster id of the enrolled device that
registered it (`DeviceToken.deviceId`). The server SHALL derive it at
`POST /api/push/register` from the caller's own roster entry (the bearer token's
enrolled device), NOT from the request body — a caller cannot claim another device's
id. A token registered without a resolvable roster entry (e.g. a token persisted before
this capability) SHALL have an empty `deviceId` and be treated as UNLINKED (legacy).

#### Scenario: Register stamps the device id

- **WHEN** a paired device calls `POST /api/push/register` with its bearer token
- **THEN** the stored `DeviceToken` carries that device's roster id as `deviceId`

#### Scenario: Legacy tokens are unlinked

- **WHEN** a token loaded from disk has no `deviceId` (registered before this capability)
- **THEN** it keeps authenticating/receiving pushes and is reported as UNLINKED

### Requirement: Revoking a device drops its push token

When an enrolled device (or a guest share link) is revoked, the server SHALL also
unregister every push token bound to that device id, so a removed/estranged device stops
receiving notifications immediately — without editing the on-disk token store. An empty
device id SHALL NOT match any token (a legacy revoke cannot blanket-drop unlinked tokens).

#### Scenario: Revoke stops notifications

- **WHEN** `POST /api/devices/revoke` removes a device that had registered a push token
- **THEN** that device's push token is dropped and no further push is delivered to it

#### Scenario: Legacy revoke is not a blanket drop

- **WHEN** a device with no bound token (empty id) is revoked
- **THEN** no unlinked (legacy) tokens are dropped

### Requirement: The push-token store is inspectable and clearable

The server SHALL expose the registered push-token store to the MASTER token (the Mac's
own CLI) for inspection and cleanup, and SHALL refuse any non-master caller (`403`):

- `GET /api/push/tokens` SHALL return each token REDACTED (a short prefix only, never the
  full secret) with its `deviceId`, platform, env, and kinds.
- `POST /api/push/forget` SHALL drop tokens by selector — `{deviceId}` (that device's
  tokens), `{orphans:true}` (only UNLINKED legacy tokens), or `{all:true}` (every token)
  — persist the change, and return the count removed.

The CLI SHALL surface this as `gtmux devices --push` (the roster annotated with each
device's push binding + a count of unlinked tokens) and
`gtmux devices --forget-push <device-id|orphans|all>`.

#### Scenario: Inspect never leaks the token

- **WHEN** the master token calls `GET /api/push/tokens`
- **THEN** each entry shows a redacted token prefix (not the full token) plus deviceId,
  platform, env, and kinds

#### Scenario: Clear orphaned legacy tokens

- **WHEN** the master calls `POST /api/push/forget {orphans:true}`
- **THEN** only tokens with an empty `deviceId` are removed and the store is persisted

#### Scenario: A non-master caller is refused

- **WHEN** a device or guest token calls `GET /api/push/tokens` or `POST /api/push/forget`
- **THEN** it is refused (`403`)
