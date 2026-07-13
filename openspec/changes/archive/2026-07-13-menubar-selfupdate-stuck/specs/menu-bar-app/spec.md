# menu-bar-app Specification

## MODIFIED Requirements

### Requirement: Check for updates + one-click self-update

The app SHALL check for a newer release (reusing the CLI's own `gtmux update --check`)
and offer a one-click update that reuses `gtmux update` (CLI + app), spawned DETACHED
so it survives the installer pkill'ing + relaunching the app.

The one-click update SHALL ALWAYS terminate in a defined state — **relaunched to the
new version**, or an **`updateFailed` retry** — and SHALL NOT sit on the "Updating…"
spinner forever. Concretely:

- The installer SHALL relaunch the swapped app with a **force-new-instance** launch
  (`open -n`), never a bare `open` that can re-activate a not-yet-exited old instance
  instead of launching the freshly-swapped binary. The app's newest-wins
  single-instance guard SHALL terminate any older instance so no duplicate status item
  remains.
- The detached job records its exit code. A **non-zero exit** (network blip / SHA
  mismatch) SHALL flip to `updateFailed` with a retry.
- On a recorded **exit 0**, the installer is expected to have pkill'd + relaunched the
  app; if the app is nonetheless STILL running past a short grace period, the relaunch
  did not take, and the app SHALL self-heal by comparing the on-disk installed bundle
  version to its own running version:
  - **installed version newer than running** → the swap succeeded but the relaunch was
    missed; the app SHALL force-launch the installed bundle (`open -n`) and terminate
    itself, so the new version takes over.
  - **installed version equal to running** (or unreadable) → the swap never happened
    (e.g. the app download was skipped); the app SHALL flip to `updateFailed` with a
    retry rather than spin.
- A download that wedges BEFORE any exit code is recorded SHALL still be caught by a
  hard timeout that flips to `updateFailed`.

#### Scenario: Update fails and offers retry

- **WHEN** a one-click update's download fails (network blip / SHA mismatch)
- **THEN** the app flips to an "update failed — retry" banner (not a stuck spinner),
  and tapping it re-runs the update

#### Scenario: Installer relaunch is missed but the swap succeeded

- **WHEN** the detached `gtmux update` records exit 0, but this app is still running
  past the grace period AND the on-disk `Gtmux.app` bundle version is newer than the
  running version
- **THEN** the app force-launches the installed bundle with `open -n` and terminates
  itself, so the newer version takes over (rather than spinning on "Updating…")

#### Scenario: Installer reported success but the app was never swapped

- **WHEN** the detached `gtmux update` records exit 0, but this app is still running
  past the grace period AND the on-disk `Gtmux.app` bundle version equals the running
  version (the app step was skipped)
- **THEN** the app flips to an "update failed — retry" banner rather than spinning on
  "Updating…"
