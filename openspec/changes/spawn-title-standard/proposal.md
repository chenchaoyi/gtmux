# spawn-title-standard

## Why

When HQ spawns agent windows with `gtmux spawn`, the window/session name defaulted to a
weak derivation (first few words of the goal), and the spawn report gave `%pane
(session)` — no tmux window NUMBER and no purpose title. So a fleet of spawned windows
was hard to tell apart and hard to reference for a jump. The user asked for a standard:
titles that (1) name the window's PURPOSE concisely, and (2) expose the window's tmux
number.

The window number can't be baked into the name — the user runs `renumber-windows on`, so
a static number goes stale when any window closes. The tmux number is instead the LIVE
locator `session:window.pane`, which the radar already computes each poll.

## What Changes

- `gtmux spawn` reports the **standard handle** `<loc> (%pane) · <title>` on success (and
  adds `loc` + `title` to `--json`): `loc` is the live `session:window.pane` (the window
  number, correct under renumber-windows), `title` is the purpose slug (the pane title).
- The **HQ playbook** gains a window-title standard (playbook v10): always pass a concise
  `--title` (verb-object kebab, ≤~24 chars) naming the purpose, and always refer to a
  spawned window by its `loc %pane · title` handle so the user can jump by number.

## Impact

- Spec: `agent-dispatch` (new requirement for the standardized handle).
- Code: `internal/app/spawn.go` (`spawnHandle`/`spawnLocator`, report + `spawnJSON`),
  `internal/hq/hq.go` (playbook text + `hqPlaybookVersion` 9→10).
- Docs: `docs/cli.md` `gtmux spawn`.
- Back-compat: additive — the JSON gains optional fields; the human report is reworded.
  The playbook auto-upgrades on the next `gtmux hq` (versioned-hq-playbook).
