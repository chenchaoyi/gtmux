# Design — supervisor MVP

## The three-tier split (and what P1 builds)

```
决策层  supervisor agent (gtmux hq)     ← P1: launchable, pull-based
摘要层  deterministic digest             ← P1: gtmux digest / /api/digest
事件层  hooks + radar + transcript      ← exists (waiting/working/idle, resume, bg, errored)
```

The cognition itself lives in the SUPERVISOR (an LLM agent), not in gtmux core.
Core stays cgo-free, token-free, deterministic; it serves structured raw
material. This is the "短状态几十 token 常驻 / 深摘要按需" cost design from the
2026-07 discussion: `digest` rows are tiny and always fresh; deep dives are the
supervisor running `tmux capture-pane`/reading the transcript only when a row
warrants it (event-worthy: waiting / errored / stuck), never bulk re-reading.

## Digest is assembly, not inference

Every field already exists in a store gtmux owns; `internal/digest` only joins:

| field | source |
|---|---|
| pane/loc/agent/source/state/since | `agents --json` gather (incl. native rows) |
| kind (permission/plan/question) | waiting marker kind |
| goal | transcript: last USER prompt (trimmed) |
| last | transcript: tail of last assistant reply |
| ask | `prompt.ParseOptions(capture-pane)` when waiting |
| error / bg | errored-idle + background markers |
| project/branch | radar's gitInfo |

No new persistence: compute on demand (CLI/API call), riding the transcript
loader's incremental tail cache. If a session has no transcript, fields degrade
to "" — the row still renders from radar signals alone (zero-intrusion default;
the optional STATUS-block convention from the discussion stays P2+).

## The supervisor is "just an agent" — on purpose

`gtmux hq` = `gtmux new`-style spawn into `~/.config/gtmux/hq/` where a generated
CLAUDE.md (user-editable, never overwritten once present) teaches the loop:
digest → judge → drill down → drive (`gtmux send`) → report. Because it's a
normal tmux agent: it shows in the radar, jump/notifications work, the phone can
talk to it, and its home dir gives persistent cross-session memory (the
knowledge-accumulation ask) with zero new machinery. Detection for
`role:"supervisor"` keys on the pane cwd == hq home (session-rename-proof).

Rejected alternatives: (a) baking an LLM into gtmux core — breaks cgo-free/cost
model, duplicates what agents do better; (b) a bespoke supervisor daemon +
protocol — the tmux-orchestrator path; heavier, and the discussion's verdict was
tmux+thin-convention wins for solo dev ergonomics; (c) tmux window as the
protocol — no: tmux is transport/visibility; structure rides in digest JSON.

## Naming & scope guards

Command `gtmux hq`, concept 中控 (supervisor). The radar stays the product spine
(D2 stance): hq is additive, opt-in, and P1 ships no orchestration — driving
stays human-in-the-loop via the supervisor's replies. Transport note: everything
the supervisor touches goes through gtmux/tmux CLI today; if multi-host mesh
lands later, digest/send already have HTTP twins, so the supervisor prompt—not
its architecture—changes.

## Open questions deliberately deferred

- P2 nudge: waiting-event → inject a short "[gtmux] %N waiting…" turn into the
  hq pane (re-uses send). Needs care re: interrupting mid-turn.
- P2 surfaces: 中控 card in menu-bar/mobile fed by `/api/digest`.
- P3 worktree parallelism, cross-model dispatch, shared STATUS convention.
