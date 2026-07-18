# Design: decompose-app-package

## The target dependency graph (acyclic, top → bottom)

```
                internal/app  (CLI dispatch + command shims + spawn/send/serve/tunnel)
                   │   │  │
        ┌──────────┘   │  └──────────────┐
        ▼              ▼                  ▼
  internal/hq ───▶ internal/radar   internal/dispatchbridge
        │   │           │                  │
        │   └───────────┼──────────────────┘
        ▼               ▼
  hqwake/hqnudge/    tmux · dispatch · prompt · resume · state · native ·
  hqfeed · hqpane    transcript · events · i18n   (leaves)
```

STRICT import rules (a violation is a compile error — the whole point):
- **`radar` imports ONLY leaf packages** (tmux, dispatch, prompt, resume, state, native,
  transcript, i18n). NEVER `hq`, NEVER `app`, NEVER `dispatchbridge`.
- **`dispatchbridge` imports ONLY leaves** (dispatch, events, tmux). NEVER `hq`/`app`/`radar`.
- **`hq` imports `radar` + `dispatchbridge` + hqwake/hqnudge/hqfeed/hqpane + leaves.**
  NEVER `app`.
- **`app` is the top** — imports `radar`, `hq`, `dispatchbridge`. Nothing imports `app`.

## Cycle analysis (grounded — the reason the order is what it is)

Verified against the current code:

1. **`agents.go` (the radar kernel) references NO HQ-cluster symbol** (`findHQPane`,
   `nudgeHQ`, `hqwake.*`, `distill`, `selfCheck`, …) — grep is empty. Its only imports
   are leaves. So **radar can be extracted first with no cycle.** The lone HQ *concept*
   in radar — `roleForCwd` stamping `role:"supervisor"` — compares the pane cwd against
   the hq-home PATH (via `state`), not the `hq` package, so it stays in radar cleanly.

2. **`hq.go` uses `dispatchIO`/`deliverOpts`** (today in `dispatchbridge.go` inside
   `app`). If `hq` were extracted while those stayed in `app`, `hq → app` would be a
   cycle. → `dispatchbridge` MUST become a leaf package BEFORE `hq` is extracted.

3. **`gatherDigest` is used by `slowtick.go` (an hq-cluster file)** as well as `digest.go`
   / `usagecmd.go`. So the digest producer must be in `radar` (below `hq`), not left in
   `app`, or `hq → app` recurs. → digest/usage producers go into `radar` in step 1.

4. **`findHQPane`** is used by both the hq cluster (hq/distill/selfcheck/slowtick/watchdog)
   AND app's spawn/send/serve. It resolves via the existing `internal/hqpane`. → move its
   logic into `internal/hqpane` (a leaf) so both `hq` and `app` call `hqpane.*` — neither
   depends on the other for pane resolution.

## Package contents (what moves where)

**`internal/radar`** (step 1): `gatherAgents`, `classifyAgent`, `agentPane` (→ exported
`Pane`), `roleForCwd`, `resolveWaiting`, `waitMarkStale`, `nativePanes`, CPU helpers, git
/project/branch helpers, sort helpers, the `Agent` struct + `agents --json` marshaling,
`gatherDigest`, `gatherUsage`, the digest JSON shapes. Its tests (`agents_test.go`,
`digest_test.go`, …) move with it, and a new `paneLister`-injection seam lets the
gather/assemble logic be fixture-tested (the 25.8% → up lever).

**`internal/dispatchbridge`** (step 2): `dispatchIO`, `deliverOpts`, `hookEquipped`,
`hookAgents`, `eventsForPane`, `waitAgentReady`, `pollInterval`, `shellCommands`.

**`internal/hq`** (step 3): `hq.go`, `slowtick.go`, `selfcheck.go`, `distill.go`,
`diskhygiene.go`, `tiergate.go`, `watchdog.go`, `taskscmd.go`, `eventscmd.go`,
`hqfeedcmd.go` (+ their tests). `findHQPane` → `hqpane`.

**`internal/app`** keeps: command dispatch (`app.go`), help, `spawn.go`/`send.go`/
`reap.go`/`adopt.go`, `serve.go`, tunnel/share/pair/devices, doctor, restore, update,
hooks/agent_hooks/codex_hooks, config, agent_resume. These call `radar.*` / `hq.*` /
`dispatchbridge.*`.

## Incremental order & per-increment verification

Each increment is ONE PR, behavior-preserving, `make check` + `check-design.sh` green,
merged before the next starts. A move keeps the code identical (only the package
clause + now-exported identifiers + the consumers' `radar.`/`hq.` qualifiers change).

- **PR 1 — `internal/radar`.** Move the kernel + digest producers + their tests; export
  the symbols the ~10 consumers use; update those call sites. Add the `paneLister` seam
  + first fixture tests. Gate: `make check`, and `agents --json` / `digest --json` output
  byte-identical on a live fleet (manual smoke).
- **PR 2 — `internal/dispatchbridge`.** Move the adapter; update spawn/send/serve/hq(-to-be)
  call sites. Small, mechanical. Gate: `make check`.
- **PR 3 — `internal/hq`.** Consolidate `findHQPane` into `hqpane`; move the 10 hq files
  + tests; update `app.go`'s command dispatch to `hq.CmdHQ`/`hq.CmdDigest`/… Gate:
  `make check`; HQ wake/tick/self-check/distill behavior unchanged (the seed version and
  `hqInstructions` do NOT change — a pure move must not bump `hqPlaybookVersion`).
- **PR 4 (optional) — `internal/tunnel`.** Independent; lowest priority.

## Risks

- **Hidden shared helpers.** A package-private helper used across a proposed boundary
  surfaces as an unexpected export or a second cycle. Mitigation: extract in the order
  above (leaves first), let the compiler find each edge, and place a genuinely-shared
  helper in the lowest package that needs it (often a leaf) — never reach back up.
- **Test-only helpers** (`fakeIO`, box builders) may be referenced across the new
  boundary; move each with its primary package, and dup-then-converge only if two
  packages truly need it.
- **`app.go` dispatch churn** — the command switch gets `radar.`/`hq.` qualifiers; keep
  it a thin shim, no logic moves into the switch.
- **Do NOT let a "move" become a "rewrite"** — resist refactoring the moved code in the
  same PR; that's how a behavior-preserving move grows a regression. Deferred quality-
  sweep items land as SEPARATE follow-ups.

## Why an openspec change for a refactor

There are no spec deltas (behavior is unchanged), but the decomposition is a
cross-cutting architectural decision with a strict, non-obvious ordering (the
dispatchbridge/digest prerequisites above). Capturing the boundaries + order as a
reviewed artifact is what lets a fresh session execute it increment-by-increment without
re-deriving the cycle analysis.
