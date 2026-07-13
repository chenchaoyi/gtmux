# Tasks — Fix menu-bar self-update stuck on "Updating…"

- [x] 1. Confirm root cause across `Updater.swift` → `update.go` → `install.sh`
       (bare `open` re-activates lingering old instance; app step exits 0 on skip;
       watchdog waits forever on any recorded `exit`)
- [x] 2. Port the multipilot fix: `install.sh` app relaunch `open` → `open -n`
- [x] 3. `Updater.swift`: add pure `postExitZeroAction(secondsSinceExitZero:grace:runningVersion:installedVersion:)`
- [x] 4. `Updater.swift`: track `exitZeroAt`, apply grace + version-aware self-heal in `pollUpdate`
- [x] 5. `Updater.swift`: `relaunchInstalledApp()` (`open -n` installed bundle + terminate) + installed-path/version helpers
- [x] 6. `Updater.swift`: reset `exitZeroAt` on a fresh `install()` (retry); keep the 180s pre-exit timeout backstop
- [x] 7. Test: `postExitZeroAction` — wait (within grace) / relaunch (newer on disk) / fail (same version) / fail (unreadable)
- [x] 8. Test: `internal/app/update_test.go` asserts `install.sh` relaunches with `open -n`
- [x] 9. `menu-bar-app` spec: record the no-stuck-spinner guarantee (relaunch via `open -n`, exit:0 grace, version-aware self-heal)
- [x] 10. Gates: `make check` (Go) + `cd macapp && swift build -c release && swift test` green
- [x] 11. `openspec validate --specs --strict` green; branch → PR
