# Change: done-wake-keyed-on-awaited

## Why

When HQ drives a session via `gtmux send` (not `spawn`) and awaits its completion,
gtmux does NOT fire an immediate `done` wake — so HQ sits waiting and misses the
finish (e.g. `%85` merged `#487` and HQ was never pinged). Root cause: `wakeDone`
keys the immediate-wake-vs-defer decision **solely on `tmux.Attended(pane)`** (default
`hqWake.done: unattended`) — an attended pane's completion is tallied to the periodic
tick, only an unattended one wakes immediately. `spawn` makes an unattended session so
its completion wakes; `gtmux send` drives an existing (usually attended) pane and
registers NOTHING, so its completion is treated as "attended" and deferred to the tick.
The immediacy is keyed on "is anyone watching the pane," not on "is HQ awaiting it."

## What Changes

- **An `awaited` registry** (mirroring the `waiting/<pane>` marker): panes HQ has
  dispatched work to and is expecting a completion from. `dispatch.MarkAwaited` /
  `IsAwaited` / `ClearAwaited`, backed by an `awaited/<pane>` state marker.
- **Both `spawn` and `gtmux send` register the target as awaited** — `spawn` after a
  delivered dispatch, `gtmux send` on a landed delivery. (The unverified `POST
  /api/send` — the phone user typing — does NOT register; the bug is about HQ-awaited
  CLI dispatch.)
- **`wakeDone` keys immediacy on awaited, not just attended.** An AWAITED pane's next
  completion ALWAYS fires an immediate `done` wake — regardless of attended status or
  the `done` config's attended-defer — delivered on the existing acked/retried wake
  channel (`hqnudge`, wake-reliability). The awaited flag is cleared once the
  completion fires (and on `gone`), so it is a one-shot per dispatch. A non-awaited
  pane keeps today's behavior exactly.

## Capabilities

### Modified Capabilities

- `hq-wake-protocol` — the immediate-`done`-wake rule additionally fires for an AWAITED
  pane even when attended.
- `agent-dispatch` — `spawn` and `gtmux send` register the target pane as awaited, and
  a completion clears it.

## Non-goals

- Not changing the per-pane merge window, the tick, or the `done` config values.
- Not registering the unverified `POST /api/send` path (casual phone input).
- Not changing hook-driven waiting or the startup-gate / draft detection.
