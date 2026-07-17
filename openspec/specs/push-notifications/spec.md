# Push Notifications Specification

## Purpose

Light up the phone's lock screen when an agent needs you or finishes — even when
the app is closed and the phone is off the VPN — by turning the server's own
agent-transition alerts into APNs pushes via a stateless relay.
## Requirements
### Requirement: Device registration

The system SHALL accept `POST /api/push/register` to store a device's APNs token
on the Mac (`~/.config/gtmux/push-tokens.json`, `0600`), so alerts can be
forwarded even when the app is closed.

#### Scenario: Register a device

- **WHEN** the app obtains its APNs device token and POSTs it
- **THEN** the token is persisted and used for subsequent alerts

### Requirement: Device unregistration on server removal

The system SHALL accept `POST /api/push/unregister` to drop a device's tokens from a
Mac, so that Mac stops pushing to a phone that has removed it as a paired server: the
APNs `token` stops alerts and silent-badge pushes, and the optional `activityToken`
stops Live Activity lock-screen updates, with the Mac pushing a Live Activity `end`
so a card it was keeping alive disappears. The endpoint is idempotent (200 even if a
token was never registered) and requires at least one of `token`/`activityToken`.
Each Mac keeps its own token set, so unregistering from one paired server SHALL NOT
affect push delivery from the others. The app calls it best-effort when the user
removes a paired Mac.

#### Scenario: Remove one of several paired servers

- **WHEN** a device is paired with servers A and B and the user removes server B
- **THEN** the app POSTs the device token to B's `/api/push/unregister`
- **AND** B drops the token and stops pushing that device's alerts
- **AND** A still has the token and keeps pushing its own alerts

#### Scenario: Removed server leaves the Live Activity

- **WHEN** the user removes the server the Live Activity is tracking
- **THEN** the app POSTs its Live Activity token to that server's `/api/push/unregister`
- **AND** the server drops the activity token and pushes a Live Activity `end`
- **AND** the server no longer sends lock-screen tally updates for that device

### Requirement: Live Activity survives a serve restart

The app SHALL re-assert its CURRENT Live Activity push token whenever it (re)connects to
the serve, so lock-screen updates survive a serve restart without relaunching the app.
The serve keeps Live Activity tokens IN MEMORY (per-activity/ephemeral, not persisted
like device tokens), so a restart drops them; and the OS fires the push-token callback
only on a token CHANGE, which a restart is not — so without this re-assert an ongoing
activity would go stale. A restart drops and reopens the SSE stream, so the connection
goes offline → live, which is the trigger.

#### Scenario: Serve restart, then reconnect

- **WHEN** the serve restarts (e.g. after `gtmux update`), dropping its in-memory Live
  Activity token, and the app's connection returns to live
- **THEN** the app re-POSTs its current Live Activity token to `POST /api/push/activity`,
  and lock-screen tally updates resume — no app relaunch needed

### Requirement: Server-derived alerts drive push

The system SHALL derive `waiting`/`done` alerts from its own ~1.5s diff loop (not
by draining the notify queue) and forward each to the relay for delivery.

#### Scenario: Agent goes waiting

- **WHEN** an agent transitions any→waiting
- **THEN** a `waiting` alert is forwarded to the relay for push

### Requirement: Stateless APNs relay

The system SHALL deliver pushes through a stateless relay that holds the APNs key
(ES256 JWT + HTTP/2) and stores no device state or content — a request is only a
token + a one-line title/body + `pane`/`kind`. Secrets come from the environment
only, never the repo.

#### Scenario: Relay forwards to APNs

- **WHEN** the relay receives a `/push` with a device token and copy
- **THEN** it signs a JWT and delivers to APNs, returning ok/failure

#### Scenario: Sandbox vs production

- **WHEN** the app is a debug device build (aps-environment=development)
- **THEN** the relay MUST target sandbox APNs (`APNS_ENV=sandbox`) to match the
  token, else delivery fails

### Requirement: Push is network-independent

Push SHALL arrive over any network Apple can reach (cellular, foreign Wi-Fi),
independent of whether the phone can reach the Mac. The tunnel is only needed for
the live view.

#### Scenario: Phone off the VPN

- **WHEN** the phone cannot reach the Mac but is online
- **THEN** lock-screen alerts still arrive; tapping one deep-links to the agent
  (the live pane loads only once the phone can reach the Mac again)

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

