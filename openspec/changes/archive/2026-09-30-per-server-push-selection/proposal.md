# Choose which paired Macs may notify this phone

## Why

A phone can hold several paired Macs but only has one device-wide push switch.
APNs registrations remain on each Mac after the phone switches away. The user
cannot tell which Mac will notify them or mute one without removing the pairing.

## What changes

Put a notification switch and its sync state on each owner Mac in Servers.
Persist the choice with that pairing. Reconcile the phone's APNs token against
all owner Macs on launch, setting changes and foreground; unregister muted Macs
and all Macs when the device-wide setting is off. Keep Live Activity tokens
independent. Show a truthful pending state and retry when a Mac is unreachable.
Route quick replies only to a uniquely identified owner Mac.

## Surfaces

- **Terminal / 终端**: existing push register/unregister endpoints, no CLI change.
- **Menu bar / 菜单栏**: no change; this is a phone preference.
- **Phone / 手机**: Servers switches and sync status; Settings remains the master switch.
- **iPad**: the shared Servers and Settings screens behave the same way.
- **Web / 网页**: no change; APNs is specific to the native app.

## Verification

Store migration and re-pair preservation, selected registration and unregistration,
alternate route, failure, guest exclusion and latest-choice serialization tests;
TypeScript, lint, Jest and repository gates. No device build is part of this change.
