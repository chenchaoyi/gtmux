# Fix the menu-bar self-update stuck forever on "Updating…"

## Why

A user on the running v0.18.1 menu-bar app clicked "update", the banner flipped to
**"Updating gtmux… it will relaunch when done"**, and then it **hung there
permanently** — never finished, never relaunched (screenshot confirmed). The
v0.18.1 → v0.18.2 self-update wedged on the spinner with no recovery.

Tracing the flow (`Updater.swift` → detached `gtmux update` → `runInstaller` →
`install.sh`) surfaces **three defects that compound into a permanent spinner**:

1. **`install.sh` relaunches the app with a bare `open`, not `open -n`** (line ~275).
   The bundle swap is preceded by `pkill GtmuxBar`, but `pkill` only *sends* SIGTERM
   and returns immediately. If the old process hasn't fully exited when `open
   "…/Gtmux.app"` runs, LaunchServices sees the still-registered `com.gtmux.menubar`
   instance and just **activates the dying old one instead of launching the swapped
   binary** — so the new version never comes up. This is the exact footgun the
   sibling **multipilot-companion** app hit ("stuck in updating forever") and fixed
   by switching its relaunch to `open -n` (≥0.2.234, `Sources/MPBar/Updater.swift`):
   `-n` force-starts a *new* instance of the swapped bundle regardless of the old
   one's state.

2. **`install.sh`'s app step is best-effort and still exits 0 when it silently does
   nothing.** If the app zip download/unzip fails (a mirror lag on a just-pushed
   release, a network blip), the script takes the `else note "…"` branch and
   `install.sh` still exits **0** — the app is never pkilled, never swapped, but
   `gtmux update` reports success.

3. **The Swift watchdog treats a recorded `exit:0` as "I'll be killed any moment" and
   then waits FOREVER.** `Updater.pollUpdate()` returns immediately on *any* recorded
   exit code; on `exit:0` it does nothing further, trusting the installer to
   pkill+relaunch it. But **nothing enforces that kill** — the 180s hard-timeout
   branch is unreachable once an exit code is on disk. So any path where `gtmux
   update` returns 0 **without actually terminating the running GtmuxBar** (defect 1's
   activate-the-old-instance, or defect 2's silently-skipped swap) leaves the app
   pinned on "Updating…" with no timeout, no error, no retry — permanently.

The common failure mode: **exit:0 recorded, but this GtmuxBar process is still alive.**

## What Changes

The fix makes the self-update **always terminate in a defined state** — relaunched to
the new version, or a retryable error — never a stuck spinner. Three prongs:

- **`install.sh`: relaunch with `open -n`** (port the multipilot fix). The bare `open`
  on the app-swap path becomes `open -n "${APP_DIR}/Gtmux.app"`, so the freshly-swapped
  binary is force-launched even if the old instance lingers. The app **already has a
  newest-wins single-instance guard** (`AppDelegate.terminateOtherInstances()`), so the
  new instance cleanly terminates any old one — no double status item.

- **`macapp/Updater.swift`: bound the `exit:0` wait + version-aware self-heal.** Once
  `exit:0` is recorded, start a short **grace** (~12s — a successful relaunch kills us
  within ~1–2s). If we're still `.updating` past the grace, the relaunch didn't take,
  so decide by comparing the **on-disk installed bundle version** to our running
  version:
  - **installed ≠ running** → the swap succeeded, we just weren't relaunched (defect 1):
    self-heal by `open -n`-ing the installed app and terminating ourselves. The new
    (newer) instance's single-instance guard finishes the handover.
  - **installed == running** (or unreadable) → the swap never happened (defect 2):
    flip to `.updateFailed` with a retry, exactly like a non-zero exit.

  The decision is a **pure, unit-tested function** (`postExitZeroAction`), mirroring
  multipilot's testable `duplicateInstancePIDs`. The existing 180s hard timeout stays
  as the backstop for the pre-`exit` window (a download that wedges before the status
  file is even written).

- **Guarantee an explicit terminal state.** After this change every self-update path
  ends in exactly one of: **relaunched to the new version** · **self-relaunched** ·
  **`.updateFailed` with one-tap retry**. No branch can sit on the spinner forever.

### Design forks (resolved with the user)

- **On `exit:0`-still-alive-past-grace → version-aware self-heal** (chosen over
  always-relaunch or always-fail): relaunch only when the on-disk bundle is genuinely
  newer; otherwise surface a retry. Most precise — never silently "relaunches" the
  same version and calls it an update, never strands the app.
- **Single-instance guard**: the user asked to add one — but gtmux **already has it**
  (`terminateOtherInstances()`, newest-wins). No new guard is built; `open -n` leans on
  the existing one. (Noted so we don't duplicate it.)

Out of scope: reworking `install.sh`'s app step to hard-fail (return non-zero) when the
app download fails — a manual `gtmux update` legitimately wants "CLI updated, app can
retry later", and the Swift-side version check already distinguishes this case for the
menu-bar path. Left as-is.

## Impact

- Affected code: `install.sh` (one `open` → `open -n`), `macapp/Sources/GtmuxBar/Updater.swift`
  (grace + self-heal + pure decision fn + version helpers).
- Affected specs: `menu-bar-app` (the "Check for updates + one-click self-update"
  requirement — no-stuck-spinner guarantee made explicit).
- Affected tests: `macapp/Tests/GtmuxBarTests/ModelTests.swift` (`postExitZeroAction`
  cases), `internal/app/update_test.go` (assert `install.sh` relaunch uses `open -n`).
- Affected docs/memory: `menubar-self-update` memory updated after merge.
- No CLI / `agents --json` contract change; the `gtmux update --check --json` shape is
  untouched. Menu-bar + installer only.
