# Design: done-wake-keyed-on-awaited

## Root cause (confirmed in code)

`internal/hook/nudge.go` `wakeDone`:

```go
cfg := hqwake.Load()
if cfg.Done == hqwake.DoneTick || (cfg.Done == hqwake.DoneUnattended && tmux.Attended(pane)) {
    hqwake.AddOutcome("done") // deferred to the summary tick — NOT an immediate wake
    return
}
// …build the line, deliver immediately via the acked hqnudge channel…
```

The immediate-vs-defer decision reads ONLY `tmux.Attended(pane)`. `TaskForPane` is
consulted elsewhere in `wakeDone` (the draft-suppression guard) but NOT here. So:

- `spawn` → a new, unattended session → `!Attended` → immediate wake. ✓
- `gtmux send` (`internal/app/send.go`) drives an existing pane and calls NO
  `AddTask`/registration; that pane is usually attended → deferred to the tick → HQ,
  awaiting it, gets no prompt signal. ✗ (the bug)

## Decisions

### D1 — An `awaited` marker, mirroring `waiting/<pane>`

`state.WaitingPath(pane)` already models "this pane is blocked on the user." Add the
parallel `state.AwaitedPath(pane)` = `awaited/<pane>`, and thin `dispatch` wrappers:

```go
func MarkAwaited(pane string)  // state.WriteMarker(AwaitedPath(pane), …)
func IsAwaited(pane string) bool
func ClearAwaited(pane string)
```

A marker, not a ledger Task, because a `gtmux send` to an existing pane is not a
reapable dispatch (no session/worktree) — overloading the reap ledger would be wrong.
The marker is a one-shot "HQ is expecting this pane to finish."

### D2 — Register on both dispatch paths

- `spawn` (`cmdSpawn`): after a delivered dispatch (`res.Delivered`), `MarkAwaited(pane)`
  alongside the existing `AddTask`.
- `gtmux send` (`cmdSend`, verified path): on `dispatch.StateLanded`, `MarkAwaited(pane)`.
- NOT `sendToPane` (`POST /api/send`, unverified) — the phone user typing is not an
  HQ await. NOT `--no-verify` (no landing confirmation to key on).

### D3 — `wakeDone` keys immediacy on awaited (pure, testable seam)

Extract the decision so it is unit-testable without tmux/state:

```go
// deferDone reports whether a completion should be TALLIED to the tick instead of
// waking HQ now. An AWAITED pane (HQ dispatched to it and is expecting the finish)
// ALWAYS wakes now — it overrides the attended-defer. Otherwise today's rule stands.
func deferDone(awaited bool, doneMode string, attended bool) bool {
    if awaited {
        return false
    }
    return doneMode == hqwake.DoneTick || (doneMode == hqwake.DoneUnattended && attended)
}
```

`wakeDone`:

```go
awaited := dispatch.IsAwaited(pane)
if deferDone(awaited, cfg.Done, tmux.Attended(pane)) {
    hqwake.AddOutcome("done")
    return
}
// …build line…
// deliver immediately on the acked/retried channel; clear the one-shot await
deliverWake(target, line); hqwake.StampDone(pane, now)
dispatch.ClearAwaited(pane)
```

For an awaited pane we deliver immediately (bypass the DoneDue merge-queue branch) so
the awaited completion is prompt; a non-awaited pane keeps the existing DoneDue merge
switch untouched. Delivery already rides `hqnudge` (draft-guarded, acked, retried on a
missed ack — wake-reliability), so "ack/retry" is inherited, not rebuilt.

### D4 — Clear the await on completion and on gone

`ClearAwaited(pane)` fires when the done wake is delivered (the await is satisfied — a
one-shot; a later completion won't re-fire unless HQ re-dispatches). The `gone` path
(`AddOutcome("gone")`) also clears it, so a pane that dies without finishing leaves no
stale marker. This mirrors how the `waiting/<pane>` marker is resolved.

## Risks

- **A stale awaited marker** if a dispatched pane never completes and never dies: same
  lifetime profile as the `waiting/<pane>` marker (persists until resolved) — acceptable,
  and self-heals on the next completion/gone.
- **Double-fire**: cleared on the first completion, so one dispatch → at most one
  awaited wake.
- **A send that doesn't land** never marks awaited (keyed on `StateLanded`), so a failed
  delivery doesn't leave HQ awaiting a phantom.

## Acceptance

`deferDone(awaited=true, DoneUnattended, attended=true) == false` (fires) while
`deferDone(false, DoneUnattended, attended=true) == true` (tallies). Plus the registry
round-trip and the gone-clears-it path. End-to-end: HQ `send`-drives an attended pane →
it completes → HQ receives an immediate `done` wake.

## Migration

Pure internal behavior + one new state marker dir (`awaited/`). No config or wire change.
