# Environment checks after updates

## 1. Flow

A successful CLI update first refreshes an already installed gtmux-owned DSH bridge through the new executable (`install refresh-owned --agent dsh`, bounded to 15 seconds), then invokes it with `doctor
--no-prompt --progress`, bounded to 90 seconds. Installation and health have separate
outcomes. A failed or timed-out check advises retrying `gtmux doctor`; it does not
change the install exit status. `GTMUX_ACTOR=menubar` skips this old-process check.

The relaunched Mac app runs the same owned-bridge maintenance once per app version, then `doctor --json --no-prompt` after startup. It stores the attempted version and the last valid report in UserDefaults.
A failure stays visible without an automatic retry loop; manual retry is always
available. Reports must have schema 1, consistent row counts, the current app's CLI
version and an exit status consistent with blocking findings. Invalid results never
replace a valid report.

## 2. Presentation and repair

The menu footer shows checking, incomplete check or findings. All-clear results are
available from Preferences → Diagnostics, without a persistent
banner. The report window groups the existing doctor rows, including notes and
status labels for accessibility. A stale report remains explicitly labelled when
the next check fails.

Review repairs opens `doctor --fix --terminal`. The CLI opens an interactive terminal
running the same executable with `doctor --fix`, without `--yes`. Each fix retains
its existing explanation and confirmation. Opening a terminal is not a repair
receipt; the user checks again after completing it. Some findings have no automatic
fix. Automatic maintenance is limited to recognized, already installed gtmux bridge artifacts. It preflights ownership, refuses links and foreign files, publishes complete files atomically, and verifies the result. It preserves the existing executable location, never changes user profiles, installs a third-party client, or restarts a process. Version metadata alone does not make executable-identical bridge code stale, and maintenance never downgrades a bridge from a newer release. First installation and profile changes remain explicit repair actions. A verified update shows a small caption directing Web activation to its check and explaining that existing sessions are not restarted.

## 3. Process boundaries

The Mac runner uses private temporary files instead of pipes so descendants holding
stdout cannot keep the check alive indefinitely. It reads at most 1 MiB per stream,
terminates on timeout and kills the check process if termination takes over two
seconds. Temporary files are removed. Go uses CommandContext plus WaitDelay.

This adds no HTTP endpoints or phone/web repair controls. Reports remain on the Mac.
A same-version reinstall does not repeat automatic bridge maintenance. Opening the menu or report refreshes a check older than five minutes, coalescing concurrent checks; reopening does not loop on a failure. Check again remains available immediately. These freshness/manual checks are read-only. A failed maintenance receipt does not turn installation into failure or replace a later health result; current doctor findings remain visible.

## 4. Verification

Synthetic CLI and Swift tests cover report/exit consistency, refusal of mutating
flag combinations, the new executable and timeout, report persistence, retry,
version mismatch, repair-launch failure and both output streams. No real environment
repair or running-app replacement is performed by these tests. Visual and VoiceOver
acceptance on the installed app remain separate from these automated checks.

## 5. Report readability

Every native row has a section-qualified identity. Conditional checks cannot reuse
another group's row or move expanded details to a different check. The window
uses explicit scrollable groups with the reading width limited to 760 pt.

Known JSON checks have readable labels and an optional bilingual `description`
that explains the actual measured prerequisite. Status, value and `note` remain
the check result. Unknown checks retain their original label/note; older reports
without descriptions still render. Labels are not stable machine identifiers.

The native title bar names the window; the result summary and check names use
`Theme.Font.session` (13 pt); descriptions, results and actions use the existing
12 pt task/action fonts. Each
row presents its purpose directly; its summary is a native disclosure control
for the full result and technical note, without a separate Details line. Rows use
6 pt vertical padding and groups 16 pt spacing. Long result previews wrap to two
lines; hover and the disclosure expose the full value. All text uses system-adaptive colours; warning icons use the shared amber.
The summary distinguishes blocking items, recommendations and passed checks,
and the check timestamp uses the selected app language with date and time.

## 6. Completion, classes and findings

Completion reflects the probe lifecycle. A completed report containing
recommendations remains complete; a timeout or invalid report is a separate
failure and leaves the previous report explicitly labelled. The summary shows
passed, recommended, blocking/feature-affecting and informational counts. Only
recommended and blocking rows are findings. Informational rows may represent
optional or unverified conditions; they are not counted as passed checks.

The human-readable CLI ends with every finding, ordered blocking first, preserving
its group, value and guidance. The fix pass reuses the same rendering for remaining
findings and distinguishes recommendations from application errors. Neither a
recheck nor a summary applies repairs; the original exit-code policy remains.

The Mac window ends with self-contained findings: full current values and guidance
are selectable and wrap without truncation. Cards are not links back to the same
check. Optional core-owned `action` and `command` fields offer specific actions:
open the HQ overview for board maintenance or copy the read-only error-log command.
Commands are never executed by the report. Unknown actions and older reports remain
plain readable guidance. Repair review stays a separate, interactive terminal action.

Mac check rows still show their result once; disclosure retains only distinct
guidance and the full long result. This applies to older reports as well.

## 7. Save evidence and uncertainty

Snapshot modification time records the last layout change, not necessarily the last
save execution: tmux-resurrect removes identical new layout files. The serve
backstop records script completion separately, scoped to the tmux socket/server PID
and a bounded fingerprint of a valid layout. Doctor accepts recent matching success
within the armed grace; legacy unscoped, failed, future, expired or mismatched
receipts cannot certify health. Missing/unreadable/invalid snapshots need attention.
Save receipts do not verify pane-content archives or an actual restore.

A stale layout without recent execution evidence is reported as **unconfirmed**,
not proof that saving stopped. Continuum's launch timestamp and active agent output
are not successful-save receipts. Suspected conversation binding mismatches likewise
remain unconfirmed; a newer unbound conversation in the same directory may belong to
another session. Doctor neither changes nor infers ownership from that heuristic.


## 8. Optional clients and historical failures

A missing optional dst client or absence of a connected terminal session is informational, not a permanent repair recommendation. Current verified sessions with unavailable input retain an actionable recommendation. Tool discovery honors PATH first, then known locations and installed NVM Node versions, preferring a numeric default alias. It never sources shell startup files. A resolved dst launch carries its paired Node runtime directory in that child's PATH.

Failures of past user operations are retained as informational history with a count, last time and readable operation names. Non-action diagnostic errors remain recommendations for inspection. Neither class proves an ongoing outage on its own; the report never deletes error history to appear healthy.
