# Tasks — retire-perception-spool

Status: IMPLEMENTED (2026-08-17). Deletion-heavy; the read-time gap warning landed
test-first, and the check-design retired list caught a missed TROUBLESHOOTING.md
section on its first run — the lock works.

## 1. Gap detection at the consumption point (test-first)

- [x] 1.1 `gtmux events --since-seq <n>` reads via `events.ReadSince` and, on
      gap=true, prints one CRITICAL warning line (en+zh) telling HQ to rebuild
      from a `gtmux digest` snapshot before acking. Tests: holed journal warns;
      contiguous journal does not; the warning is not JSON-polluting (`--json`
      keeps records parseable — warning goes to stderr).

## 2. The copy layer goes

- [x] 2.1 Delete `internal/hqfeed` daemon.go, tail.go, watchdog.go and the
      spool/cursor/heartbeat/pid halves of hqfeed.go (+ their tests); keep
      surface.go, config.go, and the still-raised control-record names
      (`wake-degraded`, `self-check`, `distill`). `ControlReconcile` and
      `ControlFeedDegraded` retire (writer and reader both gone).
- [x] 2.2 `gtmux hq-feed` command retires: hqfeedcmd.go deleted, app.go case
      dropped, spawnFeedDaemon gone from `gtmux hq` startup and the slow-tick.
- [x] 2.3 slowtick.go: feedWatchdog + restart gate + feed-fail counters deleted;
      wakeWatchdog and journalDegradation stay. Tests: feed-specific tests
      deleted with the behavior; wake-degraded coverage still green.

## 3. The wake class and the playbook

- [x] 3.1 `feed-degraded` retires from hqwake (class, grade, priority); the
      queue-priority list and docs/cli.md class table drop it; `wake-degraded`
      keeps its row with a copy that no longer mentions the spool daemon.
- [x] 3.2 Playbook: the `gtmux hq-feed` bullet is deleted, the perception
      teaching becomes journal + knock + pull with the read-time gap warning,
      and the self-heal discipline narrows to `wake-degraded`.
      `hqPlaybookVersion` 28 → 29.
- [x] 3.3 Retirement locked: `hq-feed` and `feed-degraded` join
      check-design's retired_check (docs tree) and the hq ban-list test
      (playbook/seeds), so the vocabulary cannot quietly return.

## 4. Docs + spec deltas

- [x] 4.1 docs/cli.md: wake-class table row, watermark section, and any
      hq-feed mention updated; CLAUDE.md command list and internal/hq file
      list drop hq-feed.
- [x] 4.2 Spec deltas: hq-attention-system (3 REMOVED, 3 MODIFIED),
      supervisor-agent (2 MODIFIED), hq-wake-protocol (2 MODIFIED).
- [x] 4.3 Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
      `scripts/check-design.sh`, `openspec validate --strict`.
