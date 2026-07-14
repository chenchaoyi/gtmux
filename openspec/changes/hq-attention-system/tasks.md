## 1. Phase ① — Perception feed foundation (seq + cursor)

- [x] 1.1 Add `Seq int64` (omitempty) to `events.Record`; assign it at `events.Append`
  from a persistent `events.seq` counter serialized with `syscall.Flock` (cgo-free);
  leave an explicitly-set seq untouched. Keep the append best-effort / fire-and-forget.
- [x] 1.2 Unit-test concurrent appends get distinct increasing seqs, seq continues across
  a rotation, and a legacy line without seq still reads (ordered by ts).
- [x] 1.3 Add a cursor-resume reader to `internal/events`: given a cursor seq, return the
  events with `seq > cursor` across both generations, sequence-ordered, plus a gap signal
  (a hole between cursor and the next available seq). Unit-test replay-exactly-once and
  gap detection.
- [x] 1.4 Thread the seq through `gtmux events --json` (additive field) and confirm the
  bare/`--follow`/`--severity` paths still pass their existing tests.

## 2. Phase ① — Perception feed daemon + watchdog

- [x] 2.1 New `internal/hqfeed`: spool path helpers under `~/.local/share/gtmux/hq-feed/`
  (`pid`, `cursor`, `heartbeat`, `spool.jsonl`), a rotated spool writer (size cap, one
  retained generation — mirror `internal/events` rotation), and cursor read/write.
- [x] 2.2 Daemon loop: tail the journal from the cursor (reuse `events.Follow` +
  `state`-backed cursor), append each event to the spool, advance the cursor, write a
  heartbeat every 30 s. Pidfile-guard singleton; a second `--daemon` is a no-op.
- [x] 2.3 Gap → reconciliation: on a cursor gap, write a `feed-degraded`/reconcile control
  record (marked `important`) to the spool. Startup reconciliation: on (re)start, replay
  from cursor + emit a `reconcile` control record so HQ pulls one `digest` snapshot.
- [x] 2.4 `gtmux hq-feed` command: `--daemon` (run the daemon), `--tail` (cursor/rotation-
  aware spool subscription HQ backgrounds), `--status` (health for the doctor). Register
  in `app.Run` + `--help` bilingual. Cgo-free.
- [x] 2.5 Watchdog in `slowTickEval`: only when `findHQPane() != ""`, check pidfile-alive
  AND heartbeat age ≤ 90 s; else restart the daemon (fork `gtmux hq-feed --daemon`) and
  count the attempt; clear the count on a fresh heartbeat. Pure decision function
  (`feedHealthy`/`shouldRestartFeed`) unit-tested without tmux.
- [x] 2.6 Degradation → CRITICAL: after two consecutive failed restarts (or a gap), write
  the `feed-degraded` control record AND one `hqnudge.Deliver` to the HQ pane, deduped
  by-tier via `markerChanged` so recovery/steady-state doesn't re-alert. Unit-test the
  dedup + the two-failure escalation boundary.

## 3. Phase ① — Stop force-typing low-value nudges

- [x] 3.1 Gate the hook's visible supervisor nudges (`nudgeSupervisor`/`nudgeResolved`/
  the QUIET-tier `[gtmux] …` lines) so gtmux no longer types `routine`/QUIET events into
  the HQ pane; keep CRITICAL/degradation as the only gtmux-originated visible lines. Keep
  the events append (the silent feed) unchanged. Adjust affected hook tests.
- [x] 3.2 Update the `session-events` + `supervisor-agent` specs' cross-refs already
  landed in this change; verify `openspec validate --strict` passes.

## 4. Phase ② — Severity → surfacing map + attention ledger

- [x] 4.1 Add a surfacing-tier mapping (`important`→CRITICAL, `notable`→NORMAL,
  `routine`→QUIET) in a small pure helper; unit-test it against representative records.
- [x] 4.2 Extend `dispatch.Task` with additive `Tier`, `Priority`, `Surfaced`+`SurfacedAt`,
  `Disposition`, `FirstSeen`, `LastUpdate`, `Archived`+`ArchivedAt` (all omitempty; legacy
  entries still load). Unit-test round-trip + legacy-load.
- [x] 4.3 Ledger ops: set/raise priority (re-order), promote (late promotion, no
  duplicate), mark surfaced/disposition, archive a closed entry (move under
  `tasks/archive/`). Unit-test each.
- [x] 4.4 `gtmux tasks --verbose`: show archived entries + surfaced/disposition columns;
  keep the default view to live entries, needs-you-first. Bilingual help. Adjust tests.

## 5. Phase ③ — Surfacing config + `gtmux quiet`

- [ ] 5.1 `config.json`: `surfaceTier` (`critical|normal|quiet`, default `normal`) + a
  `quiet` bool; a resolver (env-overridable, like `agentProxy`) returning the effective
  threshold. Unit-test resolution + default.
- [ ] 5.2 `gtmux quiet [on|off|status]` command (front door to `surfaceTier`), registered
  in `app.Run` with bilingual help. Expose the resolved threshold for HQ to read.
- [ ] 5.3 Guarantee a degradation CRITICAL is never suppressed by the threshold (test).

## 6. Phase ④ — Self-check triggers + HQ self-maintenance

- [ ] 6.1 Slow-tick self-check sensor (no LLM): raise a `self-check` control record to the
  feed when idle ≥ ~2 h with nothing surfaced AND ≥ ~12 h since last self-check; OR a
  threshold trips (open ledger > cap / journal over ceiling / cursor gap); OR ≥ 24 h daily
  floor. Rate-limit ≤ 1/h via a marker. Pure decision function unit-tested.
- [ ] 6.2 Persist last-self-check + last-surface timestamps the sensor needs (state
  markers). Unit-test the rate-limit + idle/threshold/daily branches.

## 7. HQ seed playbook (behavior — single source)

- [ ] 7.1 Update `internal/app/hq.go` `hqInstructions` (AGENTS.md): teach HQ to (a)
  background-subscribe to `gtmux hq-feed --tail` as the silent feed, (b) gate its OWN
  output by surfacing tier / the resolved `gtmux quiet` threshold (print CRITICAL/NORMAL,
  ledger-only QUIET), (c) on a `feed-degraded` control record always surface CRITICAL, and
  (d) on a `self-check` control record run the §8 self-maintenance and brief only on real
  action. Keep bilingual, terse.
- [ ] 7.2 Note in tasks + a memory that the LIVE hq home is seed-once — the commander's
  existing HQ needs a deliberate re-seed to pick up 7.1 (fresh homes get it automatically).

## 8. Gate, specs, docs

- [ ] 8.1 `make check` green (gofmt + vet + staticcheck + `go test -race`);
  `CGO_ENABLED=0 go build ./cmd/gtmux` passes.
- [ ] 8.2 `npx @fission-ai/openspec validate hq-attention-system --strict` passes; keep
  `tasks.md` checkboxes truthful as phases land.
- [ ] 8.3 Update `CLAUDE.md` (command list + a one-liner on the attention system) and
  `docs/TROUBLESHOOTING.md` (a feed-degraded entry: symptom → check heartbeat/pidfile →
  restart). Sync-specs + archive this change when all phases merge.
