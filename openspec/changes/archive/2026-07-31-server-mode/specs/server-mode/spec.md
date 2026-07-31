# server-mode — delta

## ADDED Requirements

### Requirement: Escalation is local and interactive; de-escalation is always free

Server mode SHALL be governed by an asymmetry: turning sleep OFF (escalation) SHALL
require a local, interactive administrator authorization performed at the machine, and
SHALL NOT be reachable from any remote or unattended path. Turning sleep back ON
(de-escalation) SHALL require no authorization, SHALL be possible from any surface, and
SHALL be performable automatically by a component that does not depend on gtmux running.
No component gtmux installs SHALL expose an interface that can be asked to escalate.

#### Scenario: Enabling from a headless session

- **WHEN** `gtmux server-mode on --tier clamshell` is run over SSH with no GUI session
- **THEN** it fails with an explicit message that enabling requires an administrator
  authorization at the machine, and the system's sleep settings are left unchanged

#### Scenario: Disabling needs no authorization

- **WHEN** `gtmux server-mode off` is run while server mode is on
- **THEN** sleep is restored with no password prompt, and the status reports
  `state:"off"` with `last_exit.reason:"revoked"`

### Requirement: Server mode runs until the user turns it off

Server mode SHALL stay in effect, with no time limit, until the user turns it off or a
guardrail ends it. There SHALL be no expiry countdown, no renewal prompt, and no
automatic end merely because time has passed. Because nothing expires on the user's
behalf, the informed-consent moment and the persistent indicator carry that weight
instead: enabling SHALL state, before the system authorization, that server mode stays on
until it is switched off, and while it is on there SHALL be a persistent indicator whose
own affordance switches it off.

The state SHALL still be backed by a heartbeat that gtmux refreshes while it is alive —
not as an expiry, but as the liveness signal that lets the privileged guard restore sleep
when gtmux is no longer present to keep serving.

#### Scenario: Left on for days

- **WHEN** server mode has been on for several days on a machine that stays on AC power
  with gtmux running
- **THEN** it is still on, no prompt has interrupted the user, and the persistent
  indicator has been visible the whole time

#### Scenario: The user ends it

- **WHEN** the user turns server mode off from any surface
- **THEN** sleep is restored, the indicator disappears, and the status reports
  `last_exit.reason:"revoked"`

### Requirement: A de-escalation-only privileged guard

The administrator authorization that enables the `clamshell` tier SHALL, in the same
authorization, install a root-owned guard consisting of a LaunchDaemon and a script in a
root-owned directory. The guard SHALL contain NO code path that disables sleep. The guard
SHALL run at boot and on a periodic interval, and SHALL restore sleep and then remove
itself (unload the daemon and delete both files) when any of the following holds: a
revoke marker is present, the heartbeat is missing or stale, or the battery has fallen to
the charge floor. Removal of the guard SHALL be triggerable by an
unprivileged marker, so that turning server mode off or uninstalling gtmux never requires
a second administrator authorization.

Because server mode is meant to survive a restart of the machine it is serving, the guard
SHALL allow a startup grace window after boot for gtmux to come back and resume the
heartbeat; only if the heartbeat has not resumed within that window SHALL the guard treat
the boot as an abandoned state and restore sleep.

#### Scenario: The remote machine reboots

- **WHEN** a machine with server mode on reboots and gtmux's persistent service comes back
  within the startup grace window
- **THEN** server mode is still on, the heartbeat resumes, and sleep stays disabled without
  any prompt — the user's remote session survives the restart

#### Scenario: The reason for a restart-time exit survives the restart

- **WHEN** the guard restores sleep at boot because nothing resumed the heartbeat
- **THEN** the recorded reason is readable after that reboot by the next gtmux that runs,
  because a record explaining a restart is worthless if the restart erases it

#### Scenario: Reboot onto a machine where gtmux is gone

- **WHEN** a machine with server mode on reboots and no gtmux resumes the heartbeat within
  the startup grace window
- **THEN** the guard restores sleep, records `reason:"boot-reconcile"`, and removes itself

#### Scenario: gtmux is killed

- **WHEN** every gtmux process is killed while server mode is on and the heartbeat stops
- **THEN** the guard restores sleep once the heartbeat is stale, records
  `reason:"stale-heartbeat"`, and removes itself

#### Scenario: gtmux is uninstalled without turning server mode off

- **WHEN** the gtmux binary and app are removed while server mode is on
- **THEN** the guard still restores sleep on its next run and deletes its own daemon and
  script, leaving no privileged residue

#### Scenario: The guard cannot be used to escalate

- **WHEN** the guard's installed script and daemon are inspected
- **THEN** they contain no invocation that disables sleep, so the worst outcome of
  driving the guard with hostile input is that the machine sleeps

### Requirement: Refuse clearly on a platform where the mechanism is unverified

The mechanism server mode depends on is **undocumented by the vendor** — it appears in no
`pmset` manual page — and it has been verified on exactly one configuration. The system
SHALL therefore preflight the platform before requesting any authorization, and SHALL
refuse with a specific, actionable reason rather than attempting an escalation that may
silently do nothing. The preflight SHALL check, without requiring privilege: that the host
is macOS; that the sleep-disable setting name is still accepted by the power tool; and
that the kernel exposes the live readback the whole design depends on. A configuration
outside the verified set SHALL be named as unverified — not as broken and not as supported
— and SHALL be overridable only by an explicit user choice that states what is unknown.

#### Scenario: A platform that cannot support it

- **WHEN** server mode is enabled on a host where the sleep-disable setting is not
  recognized, or the kernel exposes no live readback for it
- **THEN** it refuses before any password prompt, names which check failed and what that
  means, and changes no system setting

#### Scenario: An untested macOS version

- **WHEN** server mode is enabled on a macOS version the project has not verified
- **THEN** the user is told plainly that this configuration is unverified rather than
  unsupported, is told how to verify it themselves (enable, close the lid, check that the
  machine kept serving), and may proceed deliberately

### Requirement: An enable is not complete until the effect is verified

Enabling SHALL be confirmed by reading the live kernel state back, and SHALL NOT be
reported as successful on the strength of a command having run. A privileged write that
appears to succeed but leaves the state unchanged SHALL be reported as a failure, and the
ownership stamp SHALL NOT be written. This is required because the underlying tool exits
successfully when it refuses for lack of privilege — trusting that would make gtmux claim
a Mac is being kept awake when it is not, and later claim it restored sleep when it did
not.

A partially-applied enable — the sleep setting written but the de-escalation guard absent
— SHALL NOT be left in place: with no guard, nothing would give sleep back if gtmux went
away. The system SHALL either complete the installation or undo the setting and report why.

#### Scenario: The privileged write silently does nothing

- **WHEN** the command to disable sleep completes without an error status but the live
  readback still reports sleep as enabled
- **THEN** the enable is reported as failed with that discrepancy, no ownership stamp is
  written, and the user is not told server mode is on

#### Scenario: The guard could not be installed

- **WHEN** the sleep setting is applied but the de-escalation guard cannot be installed
- **THEN** the setting is reverted and the failure is reported, rather than leaving a Mac
  that cannot sleep with nothing left to restore it

### Requirement: Turning it off never depends on a password being available

A user-initiated switch-off MAY prompt for administrator authorization when a human is
present, but the system SHALL NOT make restoring sleep *conditional* on that prompt being
answered: the guard SHALL be able to restore sleep with no authorization at all. This is a
safety property, not a convenience — the situations that most need sleep back (charge
approaching empty, gtmux crashed, the machine uninstalled, nobody at the keyboard) are
exactly the situations where no one can type a password.

#### Scenario: Nobody is there to authorize

- **WHEN** charge reaches the floor while the user is away from the machine
- **THEN** sleep is restored without any prompt, and the reason is recorded and announced

#### Scenario: The user switches it off at the machine

- **WHEN** the user turns server mode off in person
- **THEN** sleep is restored promptly, whether or not an authorization prompt was involved,
  and the state reported afterwards reflects the verified live reading

### Requirement: Battery guardrails keyed to remaining charge, not to the power source

Server mode SHALL continue to run on battery power. Carrying a closed laptop between
locations — lid shut, on battery, for minutes at a time, with agents still working — is a
core use case, so losing the power adapter SHALL NOT by itself end server mode.

What ends it is **remaining charge**, which is what actually predicts harm:

- Server mode SHALL end and sleep SHALL be restored when remaining charge falls to a floor
  (default 20%), whatever the lid state, recorded as `last_exit.reason:"battery-low"`.
- Before that floor is reached, the system SHALL warn at a higher threshold (default 30%)
  on the Mac and on any paired device, because the user is by definition away from the
  machine.
- Enabling SHALL be refused below the warn threshold, since a session that would end almost
  immediately is not worth a privileged authorization.
- A machine with no internal battery SHALL skip all of the above.

The setting is applied globally rather than per power source, so no kernel-level recovery
happens when the adapter is pulled; the privileged guard's charge polling SHALL therefore
be treated as the sole defence, and its absence SHALL be treated as a reason to restore
sleep rather than to continue.

#### Scenario: Carried between rooms

- **WHEN** the adapter is unplugged and the lid closed while server mode is on and the
  battery is well above the floor
- **THEN** server mode stays on, the machine keeps serving, and no exit is recorded —
  the walk between rooms does not interrupt the agents

#### Scenario: Charge reaches the floor

- **WHEN** remaining charge falls to the floor while running on battery
- **THEN** sleep is restored, `last_exit.reason` is `battery-low`, and the user was warned
  at the higher threshold beforehand on the Mac and on any paired device

#### Scenario: Enabling with almost no charge

- **WHEN** `gtmux server-mode on` is run on battery below the warn threshold
- **THEN** it refuses, states the remaining charge in plain language, and changes no
  system setting

### Requirement: gtmux reverts only the sleep setting it owns

gtmux SHALL stamp that it was the one that disabled sleep. When the system reports sleep
disabled but gtmux holds no ownership stamp, gtmux SHALL report the discrepancy and SHALL
NOT change the setting — the user (or another tool) may have set it deliberately. When the
stamp shows gtmux owns it and no live heartbeat exists, gtmux SHALL restore it.

#### Scenario: Someone else's setting

- **WHEN** sleep is disabled system-wide but no gtmux ownership stamp exists
- **THEN** `gtmux server-mode status` reports `owned_by_gtmux:false` alongside
  `system_disablesleep:true`, and gtmux does not modify the setting

#### Scenario: Our own state, abandoned

- **WHEN** the sleep setting carries gtmux's ownership stamp but no gtmux is left to keep
  the heartbeat alive
- **THEN** sleep is restored and the guard removes itself, because the state gtmux created
  is gtmux's to clean up

### Requirement: Two honestly-labelled keep-awake tiers

The system SHALL distinguish an unprivileged `awake` tier (a sleep assertion held for as long as
server mode is on, which prevents idle sleep only and requires the lid to stay open) from the
privileged `clamshell` tier (the lid may close). Every surface that shows server mode SHALL
show which tier is live, and SHALL NOT present the `awake` tier as surviving a closed lid.

#### Scenario: Assertion-only tier is not overstated

- **WHEN** server mode is on at the `awake` tier
- **THEN** the status and every surface report the `awake` tier and state that closing the
  lid will still sleep the machine

### Requirement: Machine-readable server-mode status

The system SHALL expose the server-mode state as a deterministic, machine-readable
document via `gtmux server-mode status --json`, carrying at least: `state`
(`on|off|ended`), `tier`, `since`, `heartbeat_at`, `power`, `battery_pct` (omitted when
there is no internal battery), `guard.installed`, `guard.healthy`,
`system_disablesleep` (the raw readback, which SHALL be read from the power-management
preferences file — the `pmset` reporting commands do not expose this setting in either
state), `owned_by_gtmux` (the ownership stamp), and
`last_exit` with `at` plus a `reason` of `revoked | battery-low | stale-heartbeat |
boot-reconcile | thermal | uninstalled | lapsed`. There SHALL be no expiry field, because server
mode does not expire. The state SHALL be derived from the machine's readback cross-checked
against gtmux's own record, never from gtmux's record alone.

#### Scenario: Status is read from the machine, not from our file

- **WHEN** gtmux's own record says server mode is on but the system readback says sleep is
  enabled
- **THEN** the status reports the disagreement rather than claiming server mode is active

### Requirement: Every exit is announced with its reason

When server mode ends for any reason other than an explicit local `off`, the system SHALL
tell the user on the Mac and on any paired device, and SHALL carry the machine-readable
`reason`. An announcement SHALL NOT be routed through the supervisor's notification path,
which stays worker-scoped.

#### Scenario: Ending while the user is away

- **WHEN** server mode auto-exits because the battery reached the floor
- **THEN** a notification on the Mac and a push to the paired phone both say server mode
  ended and why, so the user learns the closed-lid session is over

#### Scenario: The setting silently stops taking effect

- **WHEN** the readback reports sleep is enabled again while gtmux still believes server
  mode is on — the documented Apple-Silicon failure across a power-source change
- **THEN** the state is reported as lapsed rather than on, `reason:"lapsed"` is recorded,
  and the user is told, because a closed-lid session that quietly died is worse than one
  that ended loudly

### Requirement: Server mode changes exactly one power setting

The system SHALL modify only the sleep-disable setting. It SHALL NOT alter idle-sleep,
display-sleep, hibernation, standby, wake-on-LAN, screen-lock, auto-login, or any other
power or security setting, and SHALL NOT suppress the display lock that follows a closed
lid. Thermal signals SHALL be advisory in this capability's first version: the system MAY
warn about sustained load or a reported thermal level, reusing the established damped
warning discipline, but SHALL NOT silently end a session on a thermal heuristic.

#### Scenario: Other settings are left alone

- **WHEN** server mode is enabled and later ends
- **THEN** every power setting other than sleep-disable holds the value it had before, and
  the machine still locks per the user's settings when the lid closes
