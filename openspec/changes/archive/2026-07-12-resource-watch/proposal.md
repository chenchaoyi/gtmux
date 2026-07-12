# resource-watch — local machine resource monitoring, attributed to agents

## Why

gtmux HQ dispatches and drives many agents but is blind to the MACHINE they run
on. Disk fills, memory pressures, load spikes — and HQ keeps piling on. It should
sense storage/memory/CPU limits, weigh them when dispatching, and when severe give
ACTIONABLE reclaim advice or suggest holding new sessions. (User-decided
2026-07-12; local state for acceptance: disk ~40 GiB free → amber, memory 43%
free, load 7.9/14 cores.)

## What Changes

- **New capability `resource-watch`** — deterministic, cgo-free, reusing the
  usage-watch framework's shape (snapshot + layered thresholds + warn channel):
  - **Machine snapshot**: disk via `df` (the relevant volume), memory via
    `memory_pressure -Q` (its built-in normal/warn/critical maps straight to our
    tiers), CPU via loadavg ÷ ncpu.
  - **Per-agent attribution (the differentiator)**: for each radar pane, walk the
    process tree from its pane PID and sum RSS + CPU% — resource use attributed to
    a specific agent, isomorphic to token accounting. Surfaced per digest/usage row.
  - **Reclaim candidates (actionable) — heuristic + whitelist (decided)**: the
    general rule is a heavy process NOT under any live pane's tree AND not in a
    gtmux/system whitelist → an orphan candidate; a curated pattern set (iOS
    Simulator/CoreSimulator runtimes, dev servers still listening on a port,
    stray tmux servers, …) raises confidence + a specific reclaim hint. Each named
    with pid + how to reclaim, so the advice is executable, not vague.
  - **Thresholds/tiers** (config, defaults sane): disk amber/red on free %/GB,
    load ratio (load÷ncpu), memory from the memory_pressure tier. Evaluated on the
    **serve tick** (resources drift continuously, not per agent event).
- **Nudge with CORRECT dedup** — resource·warn is emitted ONLY from the serve
  tick (a single goroutine/process), never from a getter called by many callers.
  This fixes the class of bug where limits·warn fired 3× for one crossing (its
  nudge lived in `gatherUsage`, which /api/usage + the HQ card + the CLI call
  concurrently → a read-check-write race). Same-tier-same-value never re-nudges;
  this change ALSO moves limits·warn's nudge to the tick to kill the existing 3×.
- **Surfaces**: `gtmux resource [--json]`; a resource block on digest/usage and
  `/api/usage` (machine snapshot + per-agent RSS/CPU + reclaim candidates); the
  mobile HQ card + status strip show a resource line; HQ playbook: weigh resources
  when dispatching, recommend reclaim (name the orphans) or hold new sessions when
  severe.
- **P1-tail (per the draft): a pre-spawn/send watermark check** — `gtmux hq`/`new`
  (and optionally send) warn when a resource is at its red line before adding load.

## Capabilities

### New Capabilities
- `resource-watch`: machine snapshot + per-agent attribution + reclaim candidates
  + tick-driven warn + CLI/API + pre-flight check.

### Modified Capabilities
- `agent-digest`: rows gain per-agent RSS/CPU (additive).
- `supervisor-agent`: the nudge channel gains resource·warn (tick-emitted, deduped);
  the playbook weighs resources + advises reclaim/hold.

## Impact

- New: `internal/resource` (snapshot/attribution/reclaim/eval), `cmd resource`,
  the serve-tick evaluator, config keys. Touched: digest rows, the mobile HQ card,
  the limits·warn nudge (moved to the tick). cgo-free; `df`/`ps`/`memory_pressure`
  are macOS built-ins (Linux fallbacks: /proc, `free`, loadavg).
- Cost: one cheap sampling per serve tick; HQ pays tokens only when it reads.
