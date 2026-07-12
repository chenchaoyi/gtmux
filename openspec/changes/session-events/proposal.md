# session-events — a hook-fed event stream gtmux HQ (and any consumer) can subscribe to

## Why

The menu-bar/mobile apps stay aware of every session by polling `agents --json`
AND receiving pushed events (the hook→notify queue; mobile's SSE `/api/events`).
gtmux HQ already plugs into the SAME hook source — but only via three targeted
nudges (waiting / usage·warn / limits·warn) — and otherwise must PULL a snapshot
(`digest --json`). It lacks the apps' continuous, low-cost feed of ALL session
events. HQ is a terminal agent (can't hold an SSE), so its native equivalent is
a file it can `tail`. (User 2026-07-12: HQ should share a common hook/subscribe
mechanism so it can get any session's execution status.)

## What Changes

- **New capability `session-events`**: the hook APPENDS one JSON line per
  lifecycle event, for every session, to `~/.local/share/gtmux/events.jsonl` —
  the SAME event source that already feeds the state markers, the notify queue,
  and the HQ nudges (additive; none of those change). Each record:
  `{ts, event, state, pane, loc, session, agent, kind?}`. The log is SIZE-BOUNDED
  (truncate-front past a cap, like the restore log) so it never grows unbounded.
- **`gtmux events [--follow] [--json] [--since <dur>]`**: read (or `tail -f`) the
  stream — the terminal-native subscription. `GET /api/events` already exists for
  the apps (SSE); this is the CLI/agent side over the same facts.
- **HQ playbook + toolbox**: `gtmux events --follow` becomes HQ's subscription —
  it can watch the live stream of ANY session's execution (started / finished /
  errored / waiting / background) instead of only the 3 nudges + on-demand
  digest. Its knowledge/best-practices note when to tail vs snapshot.
- **P2 (deferred)**: an event ring in `/api/agents`' contract for the apps to
  show a per-session activity timeline; consumer filters (by session/agent).

## Capabilities

### New Capabilities
- `session-events`: the append-only hook event log + `gtmux events` reader/tailer.

### Modified Capabilities
- `supervisor-agent`: the playbook gains `gtmux events --follow` as the HQ
  subscription (alongside the pull-based digest + the push-based nudges).

## Impact

- Touched: `internal/hook` (append one line per event — cheap, after `decide()`),
  a small `internal/events` (append + bounded truncate + read/tail), `cmd events`,
  the hq playbook. No change to existing markers / notify / nudge / SSE.
- Cost: one appended line per hook event (negligible); HQ pays tokens only when
  it READS the tail, never for the stream sitting there.
- cgo-free; the record shape is a stable additive contract.
