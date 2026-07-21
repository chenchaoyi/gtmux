# hq-meta-layer

## Why

HQ (the supervisor / 参谋长) is meant to be a META layer that REDUCES the noise of
watching a fleet. But it is itself an agent session with the gtmux hook installed, and
two places still treat it like any worker:

1. **Notifications aren't role-gated.** The display layer correctly excludes the
   supervisor (menu-bar + mobile counts, sections, fleet pips), but the notification /
   push / lockscreen layer does not: `internal/hook/hook.go` fires HQ's `done` and
   `input` notifications like a worker's, and the serve fleet snapshot carries HQ into
   the lockscreen tally — so HQ can inflate the worker counts and even hijack the
   "who's waiting" headline. The session that exists to cut noise adds to it.

2. **The HQ card's "fleet pips" are redundant, and its subtitle is wrong.** The row of
   color pips (`1 red 2 cyan 1 green`) is the same status language, over the same agent
   set, as the section list right below it — anonymous dots that don't even say WHICH
   pane is waiting. And the subtitle shows the HQ pane's tmux window title (a
   claude-generated string), not a chief-of-staff conclusion. The demo hard-codes the
   ideal ("api is waiting on you · rest normal") that real data can't produce.

## What Changes

**A. Notification role-gating (Go core):**
- Add `Role` to `server.AgentStatus`; serve threads it from `radar.GatherAgents`.
- The lockscreen/SSE fleet tally EXCLUDES the supervisor from the Waiting/Working/Idle
  counts and never lets it set the "who's waiting" headline (that headline is about the
  WORKERS). A waiting alert may still fire, but HQ never corrupts the worker stats.
- The hook SUPPRESSES the supervisor's routine `done` notification (a chief-of-staff
  completing a think-cycle must not buzz you). HQ's `input` (it needs your decision) is
  kept — that's the one thing HQ SHOULD reach you for.

**B. HQ card redesign (menu-bar + mobile):**
- REMOVE the fleet pips entirely (pure redundancy with the list; keep the count only in
  the summary line).
- Replace the subtitle with a deterministic INTELLIGENCE HEADLINE synthesized from the
  fleet: name the one that needs you + "其余 N 正常", or "都正常，无需你介入" when quiet
  (red/amber when it needs you, dim when quiet). Same function both surfaces.
- Sync the mockup §12 + DESIGN/MOBILE docs (they currently mark the pips as "★ chosen").

## Impact

- Specs: `notifications` (HQ role-gating), `menu-bar-app` + `mobile-app` (HQ card).
- Code: `internal/server/events.go` (+Role, tally gate), `internal/app/serve.go` (thread
  Role), `internal/hook/hook.go` (suppress HQ done), `macapp/.../MenuView.swift` (drop
  fleetPips, headline), `mobileapp/src/ui/HQCard.tsx` (drop pips, headline), docs +
  mockup.
- Back-compat: additive `Role` field; display unchanged for workers.
- Deferred (proposed, not in this change): a DISTINCT "参谋长请你拍板" push category with
  its own sound/style (needs mobile push-kinds + relay contract work + wording input);
  and the "autonomy ledger" second line (what HQ handled for you) — both need new server
  aggregation. This change stops the pollution and fixes the card first.
