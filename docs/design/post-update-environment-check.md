# Environment checks after updates

## 1. Flow

A successful CLI update invokes the newly installed executable with `doctor
--no-prompt --progress`, bounded to 90 seconds. Installation and health have separate
outcomes. A failed or timed-out check advises retrying `gtmux doctor`; it does not
change the install exit status. `GTMUX_ACTOR=menubar` skips this old-process check.

The relaunched Mac app runs `doctor --json --no-prompt` once per app version after
startup. It stores the attempted version and the last valid report in UserDefaults.
A failure stays visible without an automatic retry loop; manual retry is always
available. Reports must have schema 1, consistent row counts, the current app's CLI
version and an exit status consistent with blocking findings. Invalid results never
replace a valid report.

## 2. Presentation and repair

The menu footer shows checking, incomplete check or findings. All-clear results are
available from the gear menu and Preferences → Diagnostics, without a persistent
banner. The report window groups the existing doctor rows, including notes and
status labels for accessibility. A stale report remains explicitly labelled when
the next check fails.

Review repairs opens `doctor --fix --terminal`. The CLI opens an interactive terminal
running the same executable with `doctor --fix`, without `--yes`. Each fix retains
its existing explanation and confirmation. Opening a terminal is not a repair
receipt; the user checks again after completing it. Some findings have no automatic
fix. No automatic installation, configuration edit or service restart is added.

## 3. Process boundaries

The Mac runner uses private temporary files instead of pipes so descendants holding
stdout cannot keep the check alive indefinitely. It reads at most 1 MiB per stream,
terminates on timeout and kills the check process if termination takes over two
seconds. Temporary files are removed. Go uses CommandContext plus WaitDelay.

This adds no HTTP endpoints or phone/web repair controls. Reports remain on the Mac.
A same-version reinstall does not trigger a second automatic check; use Check again.
The first launch with this feature also checks once. Manual checks are read-only.

## 4. Verification

Synthetic CLI and Swift tests cover report/exit consistency, refusal of mutating
flag combinations, the new executable and timeout, report persistence, retry,
version mismatch, repair-launch failure and both output streams. No real environment
repair or running-app replacement is performed by these tests. Visual and VoiceOver
acceptance on the installed app remain separate from these automated checks.
