# Tasks

## 1. Stale-save warning at restore
- [x] 1.1 Add a pure `saveStalenessWarning(lastPath string, now time.Time) string` (empty
      when fresh/absent; a bilingual "N old" message when the save mtime is >24h).
- [x] 1.2 Call it in `ensureServer` after resolving the save — print prominently
      (i18n.Sae) AND `restoreLogf` it.

## 2. serve backstop save
- [x] 2.1 New `internal/app/resurrectsave.go`: `saveIsStale(lastPath, now, threshold)`
      (pure), `resurrectSaveScript()` (mirror `resurrectRestoreScript`, save.sh),
      `driveResurrectSave(script)` (direct subprocess + `$TMUX` + `restorePATH`, then
      `sanitizeLast`), and `maybeBackstopSave()` (server-up + stale-gated, single-flight,
      async).
- [x] 2.2 Wire `maybeBackstopSave` into serve's slow tick in `serve.go` (alongside
      `hq.SlowTickEval`).

## 3. doctor autosave-armed check
- [x] 3.1 Pure `statusRightHasContinuumTrigger(sr string) bool`.
- [x] 3.2 `rowAutoSave()` — when the continuum plugin is installed, check the running
      `status-right` carries the trigger; recommend adding it if missing. Add to the
      "Restore after reboot" doctor section.

## 4. Tests
- [x] 4.1 Unit-test `saveIsStale` / `saveStalenessWarning` (fresh, stale, missing).
- [x] 4.2 Unit-test `statusRightHasContinuumTrigger` (present / absent).

## 5. Verify
- [x] 5.1 `make check` green; `scripts/check-design.sh` green (specs valid + docs).
