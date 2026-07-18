# Tasks: disk-hygiene-and-feed-backoff

## 1. Disk hygiene helpers (pure, testable)

- [x] 1.1 `trimFileTail(path, maxBytes, keepBytes)` — cap a file at maxBytes by keeping
      only its last keepBytes (dropping the partial leading line); no-op when under cap or
      missing; best-effort with an `O_APPEND` writer still attached.
- [x] 1.2 `pruneDir(dir, maxAge, maxTotal, now)` — delete entries older than maxAge, then,
      if the dir still exceeds maxTotal bytes, delete oldest-first until under the cap.
- [x] 1.3 `treeSize(dir)` + `humanBytes(n)` — recursive footprint + compact rendering for
      the doctor storage sentinel.
- [x] 1.4 Tests: trim keeps the tail + starts on a clean line + no-ops under cap;
      prune deletes by age then by size, oldest-first, and leaves a fresh small dir alone.

## 2. Wire the sweep into the serve slow-tick

- [x] 2.1 `diskHygieneSweep(now)` — time-gated to ≤ 1/30 min via a marker; caps
      `{serve,tunnel,selftunnel,restore}.log` in `state.Dir()`, prunes `uploads/`, and ages
      out the dead-pane churn markers (`frame/`, `cpu/`, `goalchanged/`, `sends/`) — never
      the digest/idle-since sources (`resume/`, `usage/`, `usagewarn/`).
- [x] 2.2 Call it from `slowTickEval` (single-writer; best-effort, no HQ nudge — this is
      housekeeping, not a perception event).
- [x] 2.3 `gtmux doctor` `Storage` row (`rowDiskUsage`) — amber ≥ 500 MB / red ≥ 2 GB,
      pointing at the likely runaway log; wired into `doctorSections`.
- [x] 2.4 Tests: the wired sweep caps all 4 logs + ages a dead marker while keeping a live
      one + respects the throttle; the doctor row's three tiers (sparse-file footprints).
- [x] 2.5 `docs/TROUBLESHOOTING.md` Disk/storage entry (root cause = unrotated launchd
      log; the sweep + doctor row; never delete `events.seq`).

## 3. Feed restart backoff + cap (pure decision)

- [x] 3.1 `hqfeed.RestartGate(attempts, now, nextAllowedAt)` — returns whether to spawn
      this tick, the updated nextAllowedAt (exponential backoff, capped), and the new
      attempt count; refuses once attempts reach the hard cap.
- [x] 3.2 Rewire `feedWatchdog` to gate `spawnFeedDaemon()` through `RestartGate`,
      persisting the attempt count + next-allowed-at markers; reset them on a healthy
      feed / no HQ. The 2-tick CRITICAL escalation is unchanged.
- [x] 3.3 Tests: first attempt immediate; backoff widens 30→60→120…capped; no attempt
      while inside the backoff window; no attempt past the cap; healthy resets the gate.

## 4. Consistency + verification

- [x] 4.1 Spec deltas (resource-watch, hq-attention-system); `openspec validate --strict`
      green.
- [x] 4.2 `make check` + `CGO_ENABLED=0 go build ./cmd/gtmux` + `check-design.sh` green.
- [ ] 4.3 Dogfood: let serve run, confirm an over-cap `serve.log` is trimmed to its tail
      and stale uploads are pruned; kill the feed daemon repeatedly and confirm restarts
      space out (backoff) and stop after the cap while the CRITICAL degradation stands.
