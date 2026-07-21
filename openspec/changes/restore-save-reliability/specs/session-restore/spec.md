# session-restore — delta

## ADDED Requirements

### Requirement: Warn when restoring from a stale save

`gtmux restore` SHALL warn the user when the tmux-resurrect save it is about to restore
is older than one day. The warning names how old the save is and that sessions created
since will not come back, and is written both to the user (stderr) and the restore log.
A fresh or absent save produces no warning. This keeps a silently-broken autosave (a save
that stopped updating) from restoring an ancient snapshot without any signal.

#### Scenario: Stale save at reboot restore

- **WHEN** the tmux server is down and `gtmux restore` resolves a resurrect save whose contents are more than a day old
- **THEN** it prints a prominent warning that the saved layout is N old and newer sessions won't restore, and logs the same, before restoring

#### Scenario: Fresh save restores quietly

- **WHEN** the resolved save is recent (updated within the last day)
- **THEN** no staleness warning is printed and restore proceeds normally

### Requirement: serve backstops the resurrect save

`gtmux serve` SHALL, on its slow tick, trigger a tmux-resurrect save ITSELF whenever a
tmux server is up AND the last resurrect save is stale (older than a short interval), so
that save freshness does not depend on tmux-continuum's autosave being correctly armed.
When the last save is already fresh (continuum is working) it SHALL do nothing. The save
SHALL run as a direct subprocess with `$TMUX` and a robust PATH (NEVER `tmux run-shell`,
which runs in the server's minimal-PATH environment, exits non-zero, and can poison the
`last` pointer with an empty save), and the `last` pointer SHALL be repaired afterward if
a save wrote an empty file. Concurrent backstop saves SHALL be prevented (single-flight).

#### Scenario: Backstop fires when continuum is dead

- **WHEN** serve's slow tick runs, a tmux server is up, and the last resurrect save is older than the backstop interval
- **THEN** serve triggers a resurrect save as a direct subprocess, keeping the save fresh even though continuum never saved

#### Scenario: Backstop is a no-op when continuum is healthy

- **WHEN** serve's slow tick runs and the last resurrect save was updated within the backstop interval
- **THEN** serve does not trigger a save (no duplicate work)
