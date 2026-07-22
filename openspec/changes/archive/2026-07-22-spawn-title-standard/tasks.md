# Tasks

## 1. Standard handle in the spawn report
- [x] 1.1 `spawnHandle(loc, pane, title)` (pure) + `spawnLocator(pane)` (live loc + pane
      title) in `spawn.go`; report `<loc> (%pane) · <title>` for landed/queued.
- [x] 1.2 Add `loc` + `title` to `spawnJSON`.
- [x] 1.3 Unit-test `spawnHandle` (full / no-title / no-loc / bare).

## 2. HQ playbook standard
- [x] 2.1 Add the window-title standard to `hqInstructions` (concise `--title` + always
      report the `loc %pane · title` handle); bump `hqPlaybookVersion` 9→10.

## 3. Docs + spec
- [x] 3.1 `docs/cli.md` `gtmux spawn`: `--title` standard + the handle + the `--json` shape.
- [x] 3.2 Sync the `agent-dispatch` spec (this change's delta) on archive.

## 4. Verify
- [x] 4.1 `make check` + `scripts/check-design.sh` green.
