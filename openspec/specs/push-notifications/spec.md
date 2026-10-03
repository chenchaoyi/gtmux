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

### Requirement: Choose notification sources per paired Mac

The phone SHALL offer an independent notification switch for each owner-paired Mac
in the Servers list, without changing which Mac is open. Existing pairings SHALL
default to enabled, and re-pairing the same Mac SHALL preserve the choice. Guest
links SHALL NOT register for owner alerts or show this switch. The device-wide
notification and alert-kind settings SHALL apply across the selected Macs.

The app SHALL reconcile the APNs token with every owner Mac on launch, after a
setting changes, and when the app returns to the foreground. An enabled Mac
receives `POST /api/push/register`; a disabled Mac receives
`POST /api/push/unregister` for the APNs token (including silent badge pushes).
If the device-wide setting is off or both alert kinds are off, all owner Macs SHALL
be unregistered.

The Live Activity SHALL follow the same choice for the open Mac. While that Mac may
not notify (its switch, the device-wide switch, or both alert kinds off), the app
SHALL NOT start or update a Live Activity for it, nor register its activity token.
When that choice turns off, the app SHALL send that Mac `POST /api/push/unregister`
with the activity token (so the Mac stops updating the card and pushes it an `end`)
and SHALL end the card locally even if the Mac cannot be reached. Turning a single
alert kind off SHALL NOT affect the Live Activity. Turning the choice back on SHALL
let the next refresh start a card and register its token, with no further step. Reconciliation
SHALL be serialized per Mac so a delayed older request cannot override the
latest choice, while an offline Mac does not block another Mac's setting.
An unreachable Mac SHALL appear as pending sync, with retry and an explicit
warning that it may still notify until it reconnects. The app MAY use alternate
addresses reported by that same pairing. It SHALL NOT silently claim success.

The Servers page SHALL show each Mac as one line: a connection dot on the open
Mac, the name, a notification bell and More options, with the row's connect target
separate from the bell. The bell SHALL be exposed to accessibility as a switch and
convey the stored preference. The address SHALL be shown in More options rather
than on the row. A second line SHALL appear only when the open Mac is connecting or
offline, or that Mac's setting is syncing or pending; the global-pause notice SHALL
appear once for the list, not per row. Only the connected Mac SHALL carry a green
connection marker, and server-mode state SHALL NOT leak across Macs. Guest links
SHALL be grouped separately and omit the bell. More options SHALL hold removal with
confirmation. Phone and iPad SHALL use the same bounded content component.

#### Scenario: A selected Mac is offline

- **WHEN** a selected Mac loses connection
- **THEN** its row shows Offline rather than Connected, without altering its
  notification preference

#### Scenario: A healthy list reads one line per Mac

- **WHEN** the phone is paired to three Macs, one connected, none syncing
- **THEN** each Mac takes one line with no address shown
- **AND** More options names the Mac and shows its address

#### Scenario: Mute one of several Macs

- **WHEN** the phone is paired to A and B and the user disables notifications from B
- **THEN** B drops this phone's APNs token while A stays registered
- **AND** switching the open Mac does not alter either preference

#### Scenario: Muting the open Mac ends its Live Activity

- **WHEN** a Live Activity is showing for the open Mac and the user turns that
  Mac's notifications off
- **THEN** the Mac is asked to drop the activity token and the card ends
- **AND** no later refresh starts a new card until notifications are turned on

#### Scenario: One alert kind off keeps the Live Activity

- **WHEN** the user turns off only the `done` alert kind
- **THEN** the Live Activity keeps updating

#### Scenario: Mac is offline when muted

- **WHEN** the phone cannot reach B to unregister its token
- **THEN** B is shown as pending sync, with a retry action
- **AND** a later foreground reconciliation retries the unregistration

#### Scenario: Notification quick reply belongs to another Mac

- **WHEN** a notification names a uniquely paired owner Mac other than the open one
- **THEN** a quick reply is sent to that Mac's pane
- **AND** an ambiguous or unknown Mac name never causes input on the open Mac
- **AND** the match uses the Mac's own name, never a name the user gave it on the
  phone

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

### Requirement: Live Activity goes stale when the server is unreachable

A Live Activity update SHALL carry a stale-date so a dead/unreachable server (which stops
pushing) lets iOS mark the lock-screen card stale on its own, and the widget SHALL render
a distinct "offline" state (dimmed + an offline marker) instead of a frozen tally that
reads as live. To keep a healthy-but-idle server from going stale between changes, the
serve SHALL re-push the last tally on a heartbeat shorter than the stale-date window. Both
the relay push (`aps.stale-date`) and the app's local updates (`ActivityContent.staleDate`)
set the date, so the killed-app (push-only) and foreground (app-driven) paths both hold.

#### Scenario: Server dies with the app closed

- **WHEN** the paired Mac stops reaching the phone (network switch, serve killed) and the
  app is backgrounded/killed, so no more Live Activity pushes arrive
- **THEN** iOS marks the activity stale at the last push's stale-date, and the widget dims
  the tally and shows an "offline" marker — rather than showing the frozen counts forever

#### Scenario: Healthy but idle server

- **WHEN** the serve is alive but the tally hasn't changed for a while (no on-change push)
- **THEN** the serve's heartbeat re-pushes the last tally within the stale-date window, so
  the card stays live (never falsely shows "offline")

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

### Requirement: The supervisor does not notify as a worker

The system SHALL NOT emit worker alerts — and therefore SHALL NOT push notifications — for
the supervisor session. The supervisor is a meta layer that the session list deliberately
does not present as one more session, and a push about something the user cannot find in
that list is a contradiction: it makes the meta layer read as broken rather than as a
layer. Suppression SHALL be scoped to the supervisor's ROLE and SHALL NOT mute workers,
and it SHALL NOT suppress the change signal that keeps the supervisor's own card live —
only the alert. A notification in the supervisor's own voice remains a separate design; it
is not this one wearing worker wording.

#### Scenario: The supervisor blocks on the user

- **WHEN** the supervisor enters a waiting state
- **THEN** no push notification is sent for it, while a worker entering the same state
  still notifies

#### Scenario: The supervisor finishes a turn

- **WHEN** the supervisor goes from working to idle
- **THEN** no completion notification is sent for it

### Requirement: A notification opens the surface its session belongs to

Tapping a notification SHALL open the same surface that opening that session from the
session list opens. A session presented by a dedicated surface SHALL NOT be opened by a
notification into the generic one, since that contradicts the distinction the list makes.

#### Scenario: Tapping a supervisor notification

- **WHEN** the user taps a notification belonging to the supervisor
- **THEN** the supervisor's own command surface opens, not the generic session detail
