# Environment checks after updates

## Why
Updating replaces the CLI and Mac app, but currently ends with a reminder to run doctor. The detached menu-bar installer kills the initiating process, so checking in that old process would lose both results and the repair entry. Users cannot tell whether the new version needs local hook/configuration repair.

## What Changes
- Add a versioned, read-only doctor JSON report and an explicit no-prompt mode.
- After a CLI install succeeds, execute doctor through the newly installed binary; keep installation success separate from health findings or check failure.
- The newly launched menu-bar app checks once per installed app version and preserves the last valid report. Show a short progress/result banner and a grouped environment-check window with a repair entry.
- Repair opens the existing interactive doctor --fix flow, which explains and asks before each action. Never automatically pass --yes.

## Surfaces
CLI / terminal: automatic post-install check, structured report and explicit read-only mode.
menubar: relaunch-owned check, result banner, preferences entry and grouped report/repair flow.
web: no corresponding native installation flow.
iPhone / phone and iPad: Mac environment repair belongs to the paired host's CLI/menu bar.

## Boundaries
No real Codex client restart, migration, global Codex config mutation, signing changes, or deployment/release in this change. Existing alternate-screen clients still require a separately reviewed switch; the 48-line current-frame correction does not establish usable terminal history.
