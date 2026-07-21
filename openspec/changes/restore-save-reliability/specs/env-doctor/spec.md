# env-doctor — delta

## ADDED Requirements

### Requirement: Check the resurrect autosave is armed

`gtmux doctor` SHALL, in its "Restore after reboot" section and only when the
tmux-continuum plugin is installed, check that the running tmux `status-right` carries
continuum's save trigger (the `continuum_save.sh` interpolation continuum relies on to
autosave). When the trigger is missing it SHALL recommend adding it, because a custom
`status-right` without it silently disables autosave — the save goes stale and a reboot
restores an ancient snapshot.

#### Scenario: Autosave trigger present

- **WHEN** the continuum plugin is installed and `status-right` contains the `continuum_save` trigger
- **THEN** doctor reports the autosave as armed (OK)

#### Scenario: Autosave trigger missing

- **WHEN** the continuum plugin is installed but `status-right` does not contain the trigger
- **THEN** doctor flags it with a recommendation to add the `continuum_save.sh` interpolation to `status-right`
