# Proposal: disk-hygiene-and-feed-backoff

## Why

Two robustness gaps surfaced while hardening HQ's self-heal path. Neither is the
event spool — that is already bounded (20 MB journal / 8 MB feed spool rotation,
monotonic seq). The real disk risk and the real "runs hot" risk are elsewhere:

1. **Unbounded disk growth from the launchd logs + the uploads dir.** The always-on
   `gtmux serve` / tunnel LaunchAgents write `StandardOutPath`/`StandardErrorPath` to
   `~/.local/share/gtmux/{serve,tunnel,selftunnel}.log`, and launchd NEVER rotates
   these — a chatty or crash-looping agent grows them without limit. The phone-upload
   sink `~/.local/share/gtmux/uploads/` is written on every `/api/upload` and is NEVER
   trimmed, so photos/files accumulate forever. These are the paths that actually eat
   disk; the spool is a red herring.

2. **The feed self-heal restarts with no backoff or cap.** `feedWatchdog` (serve
   slow-tick, every 20 s) spawns the perception daemon on EVERY tick it reads the feed
   as unhealthy. During a persistent outage (a daemon that can't come up) it respawns a
   doomed process every 20 s, forever. There is a CRITICAL escalation after 2 failed
   ticks, but nothing stops the churn — it keeps restarting indefinitely.

## What Changes

1. **Disk hygiene sweep** — a new, time-gated (≤ 1/30 min) sweep on the serve slow-tick
   (the single-writer place, so no races): cap each launchd log
   (`serve/tunnel/selftunnel/restore.log`) at a max size by keeping only its recent tail
   (the writer's `O_APPEND` keeps appending after it), prune the uploads dir by age (delete
   files older than the retention window) and then by total size (oldest-first until under
   a cap), and age out the per-pane churn markers (`frame/`, `cpu/`, `goalchanged/`,
   `sends/`) of DEAD panes (a live pane's marker mtime stays fresh → survives). A
   `gtmux doctor` `Storage` row reports the state-dir footprint and flags a runaway
   (amber ≥ 500 MB / red ≥ 2 GB). Best-effort; missing paths are no-ops. Pure,
   parameterized helpers (`trimFileTail`, `pruneDir`, `treeSize`) carry the logic so it is
   unit-tested without launchd.

2. **Feed restart backoff + cap** — the feed watchdog gains an exponential backoff gate
   (30 s → 60 → 120 … capped at 10 min) between restart attempts during ONE continuous
   outage, and STOPS attempting after a hard cap of 6 restarts, falling back to the
   CRITICAL degradation + the 5-min polling backstop rather than churning a doomed daemon
   forever. The gate resets the moment the feed is healthy again (or no HQ is live), so a
   later outage starts fresh. The 2-tick CRITICAL escalation is unchanged. The decision is
   a pure function (`hqfeed.RestartGate`) so backoff + cap are unit-tested without tmux.

## Capabilities

### Modified Capabilities
- `resource-watch`: gtmux SHALL bound its own on-disk footprint — cap the never-rotated
  launchd logs and prune the uploads sink — so a long-running install cannot fill the
  disk with its own logs/uploads.
- `hq-attention-system`: the mechanical feed self-heal SHALL back off exponentially
  between restarts and STOP after a bounded number of attempts per outage, instead of
  respawning every tick forever.

## Impact

- `internal/app/diskhygiene.go` (new — the sweep + `trimFileTail`/`pruneDir`/`treeSize`/
  `humanBytes` helpers), `internal/app/doctor.go` (the `Storage` sentinel row +
  thresholds), `internal/app/slowtick.go` (wire the sweep; the feed-watchdog backoff gate
  + its two new markers), `internal/hqfeed/watchdog.go` (the pure `RestartGate` +
  backoff/cap), `docs/TROUBLESHOOTING.md` (a Disk/storage entry).
- Tests: `internal/app/diskhygiene_test.go` (trim tail, prune by age + size, the wired
  sweep incl. marker aging, the doctor sentinel tiers),
  `internal/hqfeed/watchdog_test.go` (backoff schedule + attempt cap + reset).
- No new command, no HTTP surface, no config; reuses `state.Dir()` + the existing
  marker glue. The seed HQ playbook / charter are UNTOUCHED (no `hqPlaybookVersion` bump —
  the perception self-heal charter text stays with the separate playbook-coordination
  track).
