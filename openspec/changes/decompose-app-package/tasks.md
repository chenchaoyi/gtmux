# Tasks: decompose-app-package

> Execute in a dedicated clean session, ONE increment per PR, in this order. Each PR is
> behavior-preserving; `make check` + `scripts/check-design.sh` must be green before the
> next. A "move" keeps code identical apart from the package clause, now-exported
> identifiers, and the consumers' package qualifiers.

## 1. PR 1 — extract `internal/radar` (the pane-data kernel)

- [x] 1.1 Create `internal/radar`; move `gatherAgents`, `classifyAgent`, `agentPane`
  (→ exported `Pane`), `roleForCwd`, `resolveWaiting`, `waitMarkStale`, `nativePanes`,
  the CPU/git/project/branch/sort helpers, the `Agent` struct + `agents --json`
  marshaling, `gatherDigest`, `gatherUsage`, and the digest JSON shapes.
- [x] 1.2 Export the symbols the consumers use (`GatherAgents`, `GatherDigest`,
  `GatherUsage`, `Agent`, …); keep the exported surface MINIMAL.
- [x] 1.3 Update the ~10 consumer files (agents, digest, doctor, serve, slowtick, status,
  taskscmd, usagecmd, watch, watchdog) to call `radar.*`.
- [x] 1.4 Confirm `internal/radar` imports ONLY leaf packages (tmux/dispatch/prompt/
  resume/state/native/transcript/i18n) — no `hq`/`app`/`dispatchbridge`.
- [x] 1.5a Move the kernel's tests (`agents_test.go`, `digest_test.go`, …) into `radar`
  (splitting the render-only cases back into `app`).
- [ ] 1.5b Add a `paneLister` injection seam + fixture tests for the gather/assemble/
  ledger-join logic (the coverage lever) — a SEPARATE follow-up increment after the pure
  move lands, so PR1 stays a clean behavior-preserving move (design.md "deferred
  quality-sweep items land as separate follow-ups").
- [x] 1.6 Gate: `make check` + `check-design.sh` green; `agents --json` / `digest --json`
  byte-identical on a live fleet (manual smoke).

## 2. PR 2 — extract `internal/dispatchbridge` (prerequisite for hq)

- [x] 2.1 Create `internal/dispatchbridge`; move `dispatchIO`, `deliverOpts`,
  `hookEquipped`, `hookAgents`, `eventsForPane`, `waitAgentReady`, `pollInterval`,
  `shellCommands`.
- [x] 2.2 Confirm it imports ONLY leaves (dispatch/events/tmux). Update `spawn.go`/
  `send.go`/`serve.go` (and any current caller) to `dispatchbridge.*`.
- [x] 2.3 Gate: `make check` green.

## 3. PR 3 — extract `internal/hq` (the supervisor subsystem)

- [x] 3.1 Consolidate `findHQPane`/`findSupervisorPane` into `internal/hqpane` (a leaf),
  so both `hq` and `app` resolve the pane without a cross-dependency.
- [x] 3.2 Create `internal/hq`; move `hq.go`, `slowtick.go`, `selfcheck.go`, `distill.go`,
  `diskhygiene.go`, `tiergate.go`, `watchdog.go`, `taskscmd.go`, `eventscmd.go`,
  `hqfeedcmd.go` (+ their tests).
- [x] 3.3 Confirm `hq` imports `radar` + `dispatchbridge` + hqwake/hqnudge/hqfeed/hqpane
  + leaves — NEVER `app`.
- [x] 3.4 Update `app.go`'s command dispatch to `hq.CmdHQ`/`hq.CmdDigest`/`hq.CmdTasks`/
  `hq.CmdEvents`/`hq.CmdQuiet` (thin shims — no logic in the switch).
- [x] 3.5 The seed playbook is UNCHANGED — a pure move must NOT bump `hqPlaybookVersion`
  or edit `hqInstructions`.
- [x] 3.6 Gate: `make check` + `check-design.sh` green; HQ wake/tick/self-check/distill
  behavior unchanged (smoke: a `done` wake still lands, a tick still fires).

## 4. PR 4 (optional) — extract `internal/tunnel`

- [ ] 4.1 Move `tunnel.go` + `tunnelself.go` + `tunnelservice.go` (+ tests); update
  callers. Independent of radar/hq; lowest priority — may be dropped.
- [ ] 4.2 Gate: `make check` green.

## 5. Close-out

- [ ] 5.1 After the increments land, confirm `internal/app` is materially smaller and its
  test coverage rose (the fixture tests 1.5 unblocked).
- [ ] 5.2 Update any doc/memory that described the old flat `internal/app` layout.
- [ ] 5.3 Sync/verify this change vs the specs (no deltas expected) and archive it.
