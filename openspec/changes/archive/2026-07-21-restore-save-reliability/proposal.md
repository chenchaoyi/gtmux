# restore-save-reliability

## Why

`gtmux restore` is only as good as the tmux-resurrect save it restores. But gtmux
**blindly trusts** that save — it never checks how old it is, and it relies ENTIRELY on
tmux-continuum to keep the save fresh. When continuum silently stops autosaving (a real
incident: a custom `status-right` with no `#(…continuum_save.sh)` trigger disabled
continuum's autosave — the save went **18 days stale**, so every reboot restored an
ancient snapshot and lost every session created since, including the whole HQ supervisor
window), restore quietly restores the stale snapshot as if nothing were wrong.

Two gaps: (1) no freshness signal — the user has no idea their saves stopped; (2) save
freshness depends on the user's tmux config being correctly armed, which gtmux can't see.

## What Changes

- **Warn on a stale save at restore.** When `gtmux restore` boots the server and the
  resurrect save it's about to restore is older than a day, it prints a prominent
  warning (and logs it) — "your saved layout is N old; sessions created since won't come
  back" — instead of silently restoring an ancient snapshot.
- **`gtmux serve` backstops the save.** On its slow tick, serve triggers a resurrect
  save ITSELF whenever the last save is stale (older than ~10 min) — so save freshness no
  longer depends on tmux-continuum being correctly armed. When continuum IS working it's
  a no-op (the save is already fresh). The save runs as a DIRECT subprocess (never
  `tmux run-shell`, which exits 127 in the server's minimal PATH and poisons `last` with
  an empty file), reusing the existing restore machinery, and `sanitizeLast` repairs a
  poisoned pointer afterward as a belt-and-suspenders.
- **`gtmux doctor` flags a disarmed autosave.** The "Restore after reboot" section gains
  a check that the running `status-right` carries continuum's save trigger (when the
  continuum plugin is installed) — catching the exact misconfiguration proactively.

## Impact

- Affected specs: `session-restore` (stale-save warning + serve backstop save),
  `env-doctor` (autosave-armed check).
- Affected code: `internal/app/restore.go` (stale-save warn helper), a new
  `internal/app/resurrectsave.go` (`maybeBackstopSave` / `driveResurrectSave` /
  `resurrectSaveScript` / `saveIsStale`), `internal/app/serve.go` (call it on the slow
  tick), `internal/app/doctor.go` (`rowAutoSave`). Pure helpers unit-tested.
- Back-compat: additive. When continuum is healthy, the backstop never fires and the
  warning never prints — behavior is unchanged for a correctly-configured setup.
- Not in scope: auto-recreating the HQ session on restore (the backstop keeps HQ in the
  save once it's running; auto-launching an agent on restore is deferred).
