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
- [x] 1.5b DONE — added `paneSource` (pane-line) + `procSnapshot` injection seams to
  `radar.GatherAgents` (package vars defaulting to tmux/ps; behavior-preserving) and
  fixture tests (`radar/gather_test.go`) that drive the whole assemble/resolve/sort +
  digest ledger-join path over canned panes. The coverage lever landed: `GatherAgents`
  0% → 72.9%, `GatherDigest` → 63.2%, `classifyAgent` 93.5%, `resolveWaiting` 100%; the
  radar package sits at 62.4% (vs the entangled kernel's old 25.8% in `internal/app`).
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

- [~] 4.1 DROPPED (owner decision, 2026-07-18). The tunnel is tightly coupled to app's
  serve/pair/enroll machinery (cmdTunnel/launchctl/mintEnrollCode/pairingPayload/
  serveServiceInstall/serviceRemoveAll/tunnelURLPath cross-refs both directions); the
  extraction risk/reward is poor and the core god-package decomposition is already done.
  This step was always marked optional/may-be-dropped.
- [~] 4.2 n/a (4.1 dropped).

## 5. Close-out

- [x] 5.1 CONFIRMED `internal/app` is materially smaller: from ~51 non-test files to 40,
  with ~5.7k lines relocated into the compiler-bounded packages `internal/radar` (1758),
  `internal/hq` (3765), `internal/dispatchbridge` (170), `internal/panefocus` (49). The
  coverage lever also landed (1.5b): the extracted `radar` kernel is fixture-tested to
  62.4% (`GatherAgents` 0% → 72.9%), where the old entangled `internal/app` kernel sat at
  25.8%.
- [x] 5.2 Docs updated: CLAUDE.md now describes the radar/hq/dispatchbridge/panefocus
  layout + the acyclic import rule; DESIGN.md / api/contract.md / CLAUDE.md code-position
  table point `agents.go` → `internal/radar`; TROUBLESHOOTING.md points `diskhygiene.go` →
  `internal/hq`; check-design.sh PLAYBOOK path → `internal/hq/hq.go`; a memory records the
  new layout.
- [ ] 5.3 DEFERRED archive (owner decision): the 3 core move PRs (#494/#495/#496) are
  merged, but 1.5b (paneLister fixture-test seam — the coverage lever) is an intentional
  open follow-up, so the change stays in `changes/` rather than archiving with an unchecked
  task. Archive once 1.5b lands or is formally dropped. No spec deltas expected.
