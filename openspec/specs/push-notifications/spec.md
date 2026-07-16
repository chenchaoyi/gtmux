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
