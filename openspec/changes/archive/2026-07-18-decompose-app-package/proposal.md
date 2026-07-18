# Change: decompose-app-package

> PLAN ONLY. This change is the reviewed decomposition PLAN for `internal/app`. It is to
> be IMPLEMENTED in a dedicated, clean session (increment by increment, one PR each) —
> not in the session that authored it. No code moves as part of authoring this change.

## Why

`internal/app` is a god-package: **~51 non-test files / ~13k lines**, with two files near
1k (`agents.go` 1006, `hq.go` 934). Unrelated concerns — the agent radar, the HQ
supervisor subsystem, the tunnel, and every CLI command — share one flat package with no
compiler-enforced boundary, so every package-private helper is reachable from everything
and coupling only grows. It also pins `internal/app` at **25.8% test coverage**: the
pane-data assemblers (`gatherAgents`, `gatherDigest`) can't be tested against fixtures
because they're entangled with the command layer.

Extracting the cohesive clusters into their own packages gives real boundaries (an
import cycle becomes a compile error, not a code-review note) and unblocks testing the
kernel against fixture panes.

## What Changes

Extract, in a STRICT incremental order dictated by the dependency graph (see design.md):

1. **`internal/radar`** — the pane-data kernel: `gatherAgents` / `classifyAgent` /
   `agentPane` / `roleForCwd` / `resolveWaiting` / `nativePanes` / the CPU/git/project
   helpers / the `Agent` (`agents --json`) + digest JSON shapes / `gatherDigest` /
   `gatherUsage`. Consumed by ~10 files today; afterwards they call `radar.*`.
2. **`internal/dispatchbridge`** — the tmux/events adapter for dispatch (`dispatchIO`,
   `deliverOpts`, `hookEquipped`, `eventsForPane`, `waitAgentReady`). This is a
   PREREQUISITE for step 3: `hq.go` uses `dispatchIO`, so it must live in a leaf both
   `app` and `hq` can import, or `hq → app` becomes a cycle.
3. **`internal/hq`** — the supervisor subsystem: `hq.go`, `slowtick.go`, `selfcheck.go`,
   `distill.go`, `diskhygiene.go`, `tiergate.go`, `watchdog.go`, `taskscmd.go`,
   `eventscmd.go`, `hqfeedcmd.go`. Imports `radar` + `dispatchbridge` + the existing
   `hqwake`/`hqnudge`/`hqfeed` + leaves. `findHQPane` consolidates into the existing
   `internal/hqpane` (a leaf) so both `hq` and `app`'s spawn/send/serve can resolve the
   pane without a cycle.
4. **(optional) `internal/tunnel`** — `tunnel.go` + `tunnelself.go` + `tunnelservice.go`,
   independent of radar/hq. Lowest priority.

`internal/app` keeps the CLI command dispatch + thin command shims that call into the
extracted packages.

## Capabilities

No capability's REQUIREMENTS change — this is a behavior-preserving package move. There
are NO spec deltas; correctness is proven by `make check` (the existing tests move with
their code) plus the new fixture tests the extraction unblocks. (Recorded here so the
review gate knows the empty `specs/` is intentional, not an omission.)

## Non-goals

- No behavior change, no CLI/JSON/HTTP contract change, no config change.
- Not rewriting the extracted code — a MOVE, not a redesign. Renames only where a
  now-exported symbol needs a package-qualified name.
- The tunnel extraction (step 4) is optional and may be dropped or deferred.
- Not fixing the deferred quality-sweep items here (sensor-scaffold unify, marker
  liveness-GC, etc.) — though the extraction is the natural place to land some of them.
