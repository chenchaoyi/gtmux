# HQ Charter — productize the supervisor from machine-local habit to portable capability

## Why

gtmux HQ works well on THIS machine because a curated playbook + a session's
self-discipline + a handful of shipped mechanisms carry it. But most of that ability
lives as "machine-local private stuff": an edited `~/.config/gtmux/hq/CLAUDE.md`, a
`notes/hq-charter.md`, lessons in one operator's head. On a teammate's workstation,
`gtmux hq` seeds a thinner default and the sharp edges (dead windows piling up, nudges
colliding with scroll/copy-mode, un-reclaimable manual windows) bite.

The user's decision: **promote HQ's general ability into the product** so anyone's
`gtmux hq` is good out-of-the-box. This change is the single unified design for that
promotion, implemented across several follow-up PRs.

## The promotion discipline (the organizing rule)

Every HQ capability is classified by ONE test — **"does it hold on another machine?"**:

- **PROMOTE → seed** (portable policy/behavior): the A–H principles, B2 dispatch
  granularity, triage philosophy, low-noise philosophy, reclaim lifecycle, and durable
  lessons — baked into the code's **agent-neutral single-source seed** (`hqInstructions`
  / `hqKnowledgeSeeds`), so a fresh `gtmux hq` seeds them. (Rides the single-source seed
  from `hq-home-seed`.)
- **PROMOTE → code** (portable mechanism): reap-by-bare-pane, nudge dedup/coalesce,
  land-verify state machine (shipped), copy-mode injection guard, dead-session watchdog,
  window/pane-title naming spec, headless subagent teardown/build, KB scaffold — shipped
  in Go with the binary.
- **KEEP local** (machine-specific): accounts, paths, network rules, and concrete
  footgun *instances* — stay in this machine's `knowledge/` + memory, never promoted.

## What Changes (by principle)

Shipped already (this change only promotes the POLICY wording into the seed, and notes
the mechanism exists): land-verify (`hq-dispatch` #393), nudge draft-guard (#394),
by-tier dedup + payload-as-DATA (#396), single-source seed (#398), dual-channel (#394),
goal/transcript system-block filter (#401), resource-watch (#390), KB scaffold.

New in this change (design here, build in follow-up PRs):

- **A/B/E/F → seed:** the role boundary (orchestrate, don't hand-run; reclaim IS HQ's
  job via reap/subagent), main-session responsiveness, **B2 dispatch granularity** (one
  self-reporting subagent per independent step; fast ops dispatched separately and
  confirmed immediately, never chained behind a slow step), low-noise, human-in-loop —
  all as agent-neutral seed policy.
- **C → code + seed:** a window/pane-title naming convention (task-named windows, one
  feature per worktree) so a glance at tmux reads the fleet; `gtmux spawn` sets the title.
- **D → code:** reap-by-bare-pane (the ledger gap), plus a dead-session/lifecycle
  watchdog that surfaces reclaimable/stuck windows.
- **G → code:** the copy-mode injection guard (nudge vs `#{pane_in_mode}`), dead-session
  detection, folded into the existing hqnudge/dispatch robustness layer.
- **H → seed + code:** the KB curation discipline (seed) + scaffold (code, shipped).

## Impact

- Specs: `supervisor-agent` (the promoted A–H seed playbook + B2 + promotion note),
  `agent-dispatch` (reap-by-pane, window-title spec, headless teardown).
- Code (follow-up PRs): `internal/app/hq.go` (seed), `internal/app/reap*.go` +
  `internal/dispatch` (reap-by-pane, titles), `internal/hqnudge` (copy-mode guard),
  a watchdog on the serve slow-tick.
- Contracts: additive (reap accepts a bare pane; spawn sets a pane title). No breaking
  change to `agents --json` / `tasks --json` / the event shape.

## Open design forks

Listed in `design.md` §Forks and surfaced to the user before implementation begins.
