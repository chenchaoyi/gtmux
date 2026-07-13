# Design — HQ Charter productization

## Promotion taxonomy (the through-line)

| Capability | Class | Landing |
|---|---|---|
| A role boundary, B/B2 responsiveness+granularity, E low-noise, F human-in-loop, H curation | seed | `hqInstructions` (agent-neutral, single-source) |
| Triage philosophy, reclaim-lifecycle policy (suggest→approve→execute) | seed | `hqInstructions` |
| Generic operating lessons (compact-before-dispatch, model choice under quota pressure) | seed | `hqKnowledgeSeeds` best-practices |
| reap-by-bare-pane, window/pane-title spec, headless teardown/build | code | `internal/app/reap*`, `internal/dispatch` |
| copy-mode injection guard, dead-session watchdog | code | `internal/hqnudge`, serve slow-tick |
| land-verify, draft-guard, by-tier dedup, dual-channel, goal-filter, KB scaffold | code (SHIPPED) | already in main |
| Accounts, paths, network, concrete footgun instances | local | machine `knowledge/` + memory |

## Mechanism designs

### M1 · reap accepts a bare pane_id (the ledger gap)
Today `gtmux reap <id>` only resolves a ledger entry; a manually-created window
(`gtmux reap %28` → "no dispatch") can't be reclaimed. Design: when the argument is a
pane id not in the ledger, derive its repo context from the pane cwd
(`git -C <cwd> rev-parse --show-toplevel` + `--abbrev-ref HEAD` + worktree detection),
then run the SAME safety gate (worktree clean + branch merged, unless `--abandon`) and
reclaim (kill window, `git worktree remove`, delete merged branch). No ledger entry
required. `--json` reports the same outcome shape.

### M2 · headless teardown/build subagent (B/B2)
Teardown needs no LLM — M1 (reap-by-pane) covers it headlessly. For heavy work that
DOES need an agent (a build, a batch edit) without parking the HQ main loop, add
`gtmux spawn --headless <goal>`: runs the agent detached (`claude -p`-style, no tmux
window), proxied + land-tracked via the events/ledger path, so HQ dispatches-and-forgets
and the main session stays the fast human-input receiver.

### M3 · window/pane-title naming spec (C)
`gtmux spawn` sets the pane/window title to a task slug so tmux is self-describing.
Slug source: explicit `--title`, else the worktree/branch slug, else a normalized goal
head. Convention (also in the seed): window/pane title = task slug (`menubar-width`),
one feature per worktree (`gtmux-wt/<slug>`), branch `feat/<slug>`.

### M4 · copy-mode injection guard (G)
The draft-guard (#394) only checks the input box is empty; it does NOT cover tmux
copy-mode. When the HQ pane is in copy/view-mode, injected keys are eaten as nav
commands (`f` → jump-forward, yellow residue) and the nudge garbles. Design: before
injecting, check `#{pane_in_mode}`; treat "in mode" exactly like a non-empty draft —
do NOT inject, queue the nudge, deliver when the pane leaves mode (or on the next
drain). Same layer as draft-guard (`internal/hqnudge`).

### M5 · dead-session / lifecycle watchdog (D/G)
On the serve slow-tick (single-writer, where resource/limits warnings already live):
surface (a) a finished dispatch OR a lingering window whose worktree is merged+clean →
`reap-suggest` (extend the existing sweep to bare panes via M1); (b) a pane stuck
(working with no output past a threshold, or waiting/error past a timeout) → an
escalation nudge. Deduped/snoozeable like every nudge. Suggest-only; never auto-reap.

### M6 · seed promotion (A–H → hqInstructions)
Rewrite the seed playbook to carry A–H + B2 as agent-neutral policy, plus the generic
operating lessons, so `gtmux hq` seeds the full charter. Rides the single-source layout
(`hq-home-seed`): AGENTS.md canonical + CLAUDE.md import. Concrete machine instances stay
in local `knowledge/`.

## PR split (implement after this design is approved)

1. **seed promotion** (M6) — pure seed/policy, agent-neutral. Lowest risk, highest
   portability value.
2. **reap-by-pane** (M1) — unblocks reclaim of manual windows.
3. **window titles + headless spawn** (M3, M2).
4. **copy-mode guard** (M4) — small, robustness.
5. **watchdog** (M5) — the largest; may sub-split (finished-lingering vs stuck-timeout).

## Forks — RESOLVED (user, 2026-07-13; all per recommendation)

- **F1 · Reclaim of manual windows → reap-by-bare-pane** (M1). reap resolves repo
  context from the pane cwd + same safety gate; no ledger required. (A later
  `adopt-dispatch` to backfill the ledger is deferred, not in this change.)
- **F2 · Headless heavy work → reap-by-pane covers teardown (no LLM) + `spawn --headless`**
  (M2) for agent-needing heavy/batch ops, detached, no tmux window, tracked. HQ never uses
  its own subagent tool for this (would block the main loop / blur the boundary).
- **F3 · Copy-mode guard → QUEUE like a draft** (M4). On `#{pane_in_mode}`, do not
  inject; queue and deliver when the pane leaves copy-mode (or next drain). Never yanks
  the user out of their scroll. Consistent with the draft-guard.
- **F4 · Watchdog scope v1 → finished/lingering (reap-suggest, incl. bare panes) AND
  stuck/timed-out (escalate)** (M5). Deduped, snoozeable, suggest-only. The full
  needs-you ledger with age+timeout is deferred to its own change.
- **F5 · Titles → `gtmux spawn` auto-sets titles + playbook convention** (M3).
- **F6 · Lessons → promote GENERIC operating lessons into the seed best-practices; keep
  concrete instances (accounts/paths/network/the exact incidents) local.**
