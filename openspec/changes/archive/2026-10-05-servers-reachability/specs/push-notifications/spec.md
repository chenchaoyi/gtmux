# push-notifications (delta)

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
An unreachable Mac SHALL appear as pending sync, with an explicit warning that it
may still notify (or may not notify yet) until the setting reaches it. The app SHALL
retry by itself when that Mac answers again, as well as on the triggers above; there
SHALL be no separate retry control, because the reader cannot make an unreachable Mac
answer. The app MAY use alternate addresses reported by that same pairing. It SHALL NOT
silently claim success.

The Servers page SHALL show each Mac as a row of exactly two lines (mobile-app,
"The Servers page says which Macs answer and which one is open"): the name with a
notification bell and More options, and a status line. The row's connect target SHALL
be separate from the bell. The bell SHALL be exposed to accessibility as a switch and
convey the stored preference. The address SHALL be shown in More options rather than
on the row. A pending setting SHALL be said on the status line, never as a line of its
own; a sync in flight SHALL NOT be shown at all, so tapping a bell or a Mac never
resizes a row. The global-pause notice SHALL appear once for the list, not per row.
Only the connected Mac SHALL carry a filled green connection marker, and server-mode
state SHALL NOT leak across Macs. Guest links SHALL be grouped separately and omit the
bell. More options SHALL hold removal with confirmation. Phone and iPad SHALL use the
same bounded content component.

#### Scenario: A selected Mac is offline

- **WHEN** a selected Mac loses connection
- **THEN** its status line says it cannot be reached rather than Connected, without
  altering its notification preference

#### Scenario: A healthy list reads one line per Mac

- **WHEN** the phone is paired to three Macs, one connected, none pending
- **THEN** each Mac takes one row of the same two lines, name and status, with no
  address shown
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
- **THEN** B's status line says it cannot be reached and that it may still notify
  until the setting reaches it, with no retry control
- **AND** the unregistration is retried when B answers again, and on a later
  foreground reconciliation

#### Scenario: Notification quick reply belongs to another Mac

- **WHEN** a notification names a uniquely paired owner Mac other than the open one
- **THEN** a quick reply is sent to that Mac's pane
- **AND** an ambiguous or unknown Mac name never causes input on the open Mac
- **AND** the match uses the Mac's own name, never a name the user gave it on the
  phone

#### Scenario: Tapping a bell does not move the list

- **WHEN** the user taps a Mac's bell and the setting is sent to every owner Mac
- **THEN** no row changes height while the requests are in flight
