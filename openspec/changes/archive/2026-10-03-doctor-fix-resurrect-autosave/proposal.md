# Repair a missing resurrect autosave trigger

## Why

Doctor detects when a custom tmux status line disables continuum autosave, but
`doctor --fix` omits that repair and incorrectly groups it with changes requiring
manual intervention. The existing advice suggests a `~` path, which continuum may
fail to recognize and then append again, causing duplicate saves.

## What changes

- Offer a consented repair when the installed continuum script exists and the
  running status line has no save trigger.
- Append the plugin's absolute path to the current status line and persist a
  guarded command in the backed-up managed tmux config block.
- Verify the live result. Keep existing and duplicate triggers unchanged.

## Surfaces

CLI doctor output and `doctor --fix`; no app protocol or UI change. The fix uses
the same Go CLI from terminal and menubar troubleshooting flows. The phone, iPad,
and web consume the server and need no UI or protocol change.

After a successful CLI update, show a doctor reminder; `--check` stays read-only.
Also reconcile app detection across doctor, its fixer, and update: a Homebrew
installation in `/Applications` must not prompt a redundant reinstall.
