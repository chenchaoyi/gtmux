# Supervisor MVP — the 中控 agent on a cognitive digest layer

## Why

With many agents running, the user still round-robins tmux windows to understand
what each one is doing; gtmux aggregates STATUS (waiting/working/idle) but not
MEANING (what is it building, where is it stuck, what does it want from me). The
user wants a 中控管家 (supervisor) agent: one place to ask "现状?" and get a real
report, and one hand that can drive the other windows (via tmux/gtmux CLI) so the
human intervenes only when needed. (2026-07-10/11 discussions, incl. a long
ChatGPT-assisted design session.)

The key architectural conclusion from that discussion: don't build a bespoke
orchestrator — build a **terminal cognitive layer as machine infrastructure**
(any agent working on this Mac produces a readable work digest by default), and
make the supervisor just the first CONSUMER of that layer. Three tiers: event
layer (exists: hooks + radar), digest layer (this change), decision layer (the
supervisor agent). Prior art (tmux-orchestrator, tmux-bridge-mcp, tmuxpulse)
covers control/comms but nobody ships the cognition layer — that's the gap and
the moat. This deliberately refines 2026-06's D2 "stay radar, don't become an
orchestrator": the radar stays the spine; the supervisor is a consumer of it,
not a worktree-launcher GUI.

## What Changes

- **New capability `agent-digest`** — a DETERMINISTIC per-agent digest (zero
  LLM tokens, cgo-free) assembled entirely from signals gtmux already has:
  transcript (goal = last user prompt, last = tail of the last reply), waiting
  kind + parsed prompt options (ask), errored/bg markers, project/branch, state
  + since. Exposed as `gtmux digest [--json]` (human table / machine JSON) and
  `GET /api/digest` (bearer-gated, additive). Cheap "短状态" per the token-cost
  design: tens of tokens per row, updated on demand; DEEP understanding is NOT
  precomputed — the supervisor drills into a pane (capture-pane / transcript)
  only when needed, event-driven not polling.
- **New capability `supervisor-agent`** — `gtmux hq` (中控): spawns (or reuses)
  a dedicated tmux session running the user's coding agent in a persistent
  supervisor home (`~/.config/gtmux/hq/`) whose generated CLAUDE.md teaches the
  toolbox: `gtmux digest --json` to see the fleet, `tmux capture-pane` to
  inspect, `gtmux send` to drive, report-then-act norms. The home dir doubles as
  the supervisor's persistent memory — the 横向知识沉淀 lands there for free.
  Supervisor rows are marked `role:"supervisor"` in `agents --json` (detected by
  cwd, rename-proof) so surfaces can pin/badge them.
- **P1 explicitly excludes** (spec'd as deferred, NOT built here): auto-nudging
  the supervisor on waiting events; parallel-worktree orchestration (一句话拆
  多 worktree); cross-model dispatch; a dedicated 中控 view on menu-bar/mobile;
  any auto-answering of permission prompts (user: not a required ability).

## Capabilities

### New Capabilities
- `agent-digest`: the deterministic cognitive digest layer + `gtmux digest` +
  `GET /api/digest`.
- `supervisor-agent`: the launchable 中控 session (`gtmux hq`), its toolbox
  contract, persistent home/memory, and radar marking.

### Modified Capabilities
- `agent-radar`: `agents --json` rows gain an additive, optional
  `role:"supervisor"` field (backward compatible; absent for normal agents).

## Impact

- New: `internal/digest/` (pure assembly over existing stores), `cmd digest`,
  `cmd hq`, serve route `/api/digest`.
- Touched: `internal/app/agents.go` (role field), `internal/server/server.go`
  (route), docs (`README`/`docs/cli.md`).
- No mobile/menu-bar UI in P1 (JSON contract lands first; surfaces follow).
- Security: the supervisor drives panes via the SAME local CLI surface the user
  already has; nothing new is exposed remotely (`/api/digest` is read-only under
  the existing bearer token).
