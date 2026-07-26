# hq-home-quarantine — the HQ home is for the supervisor only

## Why

Dogfooding incident (2026-07-26, second machine): HQ dispatched work with
`gtmux spawn` and no `--cwd`. The new session inherited HQ's own cwd — the HQ home
— so the worker's agent read the home's `AGENTS.md` (the supervisor charter),
adopted it, and started spawning workers of its own: a supervisor-impersonation
recursion. Two mechanism gaps allowed it:

1. **Nothing stops a worker from landing in the HQ home.** Spawn's default cwd is
   the caller's cwd, which for HQ IS the home.
2. **HQ identity is cwd-coarse.** Any pane (and any radar row) whose cwd matches
   the home was treated as the supervisor: mis-spawned workers vanished from the
   radar's normal listing, and `hqpane`'s first-match resolve could even deliver
   WAKES to the worker instead of the real HQ.

## What Changes

Three layers, defense in depth:

- **Spawn quarantine (the lock):** `gtmux spawn` refuses any target that resolves
  (symlink-normalized) to the HQ home — an explicit `--cwd`, the inherited cwd when
  `--cwd` is absent, or `--pane` reuse of a pane sitting there. The refusal names
  the fix (`--cwd <project dir>`).
- **Identity precision:** the `@gtmux_hq_home` stamp outranks the path fallback in
  `hqpane` resolution, and the radar grants `role:"supervisor"` ONLY to the stamped
  pane whenever one exists — a worker parked in the home stays a normal, visible
  row. The cwd fallback stands only for legacy homes with no stamped pane.
- **Charter guard (playbook v11):** the seeded playbook opens with an identity
  self-check (a dispatched worker reading the charter must not adopt it) and the
  dispatch rules require an explicit `--cwd` on every spawn.

## Impact

- Affected specs: agent-dispatch (spawn refusal), supervisor-agent (identity
  precision + playbook).
- Affected code: `internal/app/spawn.go`, `internal/hqpane`, `internal/radar/agents.go`,
  `internal/hq/hq.go` (playbook, version 10 → 11).
- No API/schema changes; radar `role` semantics narrow (supervisor = at most the
  one true HQ pane).
