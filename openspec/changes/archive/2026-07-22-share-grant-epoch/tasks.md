# Tasks

## 1. Bind grants to the tmux server
- [x] 1.1 `ShareState.PaneEpoch`; `ShareManager` epoch + `SetEpochSource`/`GrantsStale`/`StampEpoch`.
- [x] 1.2 `tmuxServerEpoch()` (server pid + process start time) injected in `serve.go`.
- [x] 1.3 Stamp on every pane-grant write (global config, `share new`, `share set`).

## 2. Fail closed at every guest gate
- [x] 2.1 attach (before spawning a PTY), `/api/send`, pane content, guest radar (empty).
- [x] 2.2 Surface `stale` in the share capability.

## 3. Tell the owner
- [x] 3.1 `gtmux share status`: `stale` in `--json` + a loud human warning to re-grant.

## 4. Tests
- [x] 4.1 stale after a server change; valid across a serve restart with the same server;
      never stale without a tmux identity; stale guest radar is empty; status flags stale.

## 5. Verify
- [x] 5.1 `make check` + `scripts/check-design.sh` green.
