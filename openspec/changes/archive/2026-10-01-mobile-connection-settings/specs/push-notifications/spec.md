## MODIFIED Requirements

### Requirement: Choose notification sources per paired Mac

The phone SHALL offer an independent notification switch for each owner-paired Mac
in the Servers list, without changing which Mac is open. Existing pairings SHALL
default to enabled, and re-pairing the same Mac SHALL preserve the choice. Guest
links SHALL NOT register for owner alerts or show this switch. The device-wide
notification and alert-kind settings SHALL apply across the selected Macs.

The app SHALL reconcile the APNs token with every owner Mac on launch, after a
setting changes, and when the app returns to the foreground. An enabled Mac
receives `POST /api/push/register`; a disabled Mac receives
`POST /api/push/unregister` for the APNs token (including silent badge pushes),
without unregistering the Live Activity token. If the device-wide setting is off
or both alert kinds are off, all owner Macs SHALL be unregistered. Reconciliation
SHALL be serialized per Mac so a delayed older request cannot override the
latest choice, while an offline Mac does not block another Mac's setting.
An unreachable Mac SHALL appear as pending sync, with retry and an explicit
warning that it may still notify until it reconnects. The app MAY use alternate
addresses reported by that same pairing. It SHALL NOT silently claim success.

The Servers page SHALL use separate cards for Macs, with name, address and
connection state apart from the notification switch. Only the connected Mac SHALL
carry a green connection marker, and server-mode state SHALL NOT leak across Macs.
The switch conveys the stored preference; pending, syncing or global-pause notices
SHALL be separate from its tap target. Guest links SHALL be grouped separately and
omit the switch. More options SHALL hold removal with confirmation. Phone and
iPad SHALL use the same bounded content component.

#### Scenario: A selected Mac is offline

- **WHEN** a selected Mac loses connection
- **THEN** its card shows Offline rather than Connected, without altering its
  notification preference

#### Scenario: Mute one of several Macs

- **WHEN** the phone is paired to A and B and the user disables notifications from B
- **THEN** B drops this phone's APNs token while A stays registered
- **AND** switching the open Mac does not alter either preference

#### Scenario: Mac is offline when muted

- **WHEN** the phone cannot reach B to unregister its token
- **THEN** B is shown as pending sync, with a retry action
- **AND** a later foreground reconciliation retries the unregistration

#### Scenario: Notification quick reply belongs to another Mac

- **WHEN** a notification names a uniquely paired owner Mac other than the open one
- **THEN** a quick reply is sent to that Mac's pane
- **AND** an ambiguous or unknown Mac name never causes input on the open Mac
