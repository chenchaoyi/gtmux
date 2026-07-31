# env-doctor — delta

## ADDED Requirements

### Requirement: Doctor reports a left-behind sleep setting and fixes only gtmux's own

The doctor SHALL check whether the machine's sleep-disable setting is applied while no
live gtmux server-mode heartbeat exists, and SHALL report it as a finding, because that state
is otherwise invisible to the user (the setting persists across reboot and leaves no
trace in the default power-settings listing when off). When gtmux's ownership stamp shows
the setting is gtmux's own, `doctor --fix` SHALL offer to restore sleep under the existing
per-change consent rule. When there is no gtmux ownership stamp, the finding SHALL be
report-only with the manual command shown, and `--fix` SHALL NOT change it. The doctor
SHALL also report whether the privileged sleep guard is installed and healthy.

#### Scenario: An orphan gtmux left behind

- **WHEN** the sleep-disable setting is applied, gtmux's stamp owns it, and no live
  heartbeat exists
- **THEN** doctor reports that the Mac will not sleep and why, and `--fix` restores sleep
  after the usual per-change consent

#### Scenario: A setting gtmux does not own

- **WHEN** the sleep-disable setting is applied but carries no gtmux ownership stamp
- **THEN** doctor reports it as informational with the manual command to undo it, and
  `--fix` leaves it untouched
