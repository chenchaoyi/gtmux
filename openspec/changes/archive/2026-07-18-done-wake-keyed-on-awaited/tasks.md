# Tasks: done-wake-keyed-on-awaited

## 1. Awaited registry

- [x] 1.1 `internal/state`: `AwaitedDir()` + `AwaitedPath(pane)` = `awaited/<pane>`
  (mirror `WaitingDir`/`WaitingPath`).
- [x] 1.2 `internal/dispatch`: `MarkAwaited(pane)` / `IsAwaited(pane)` /
  `ClearAwaited(pane)` over the marker (thin wrappers on `state.WriteMarker/Exists/Remove`).
- [x] 1.3 Unit test: mark → IsAwaited true; clear → false; clear of an unmarked pane is
  a no-op.

## 2. Register on both dispatch paths

- [x] 2.1 `cmdSpawn` (`spawn.go`): `dispatch.MarkAwaited(pane)` when `res.Delivered`.
- [x] 2.2 `cmdSend` (`send.go`), verified path: `dispatch.MarkAwaited(pane)` on
  `dispatch.StateLanded`. Leave `--no-verify` / `sendToPane` unregistered.

## 3. wakeDone keys immediacy on awaited

- [x] 3.1 `internal/hook/nudge.go`: `deferDone(awaited bool, doneMode string, attended
  bool) bool` — pure (awaited ⇒ never defer; else the existing rule).
- [x] 3.2 `wakeDone`: read `awaited := dispatch.IsAwaited(pane)`; replace the inline
  defer condition with `deferDone(...)`; when NOT deferring an AWAITED pane, deliver the
  line immediately via `deliverWake` (acked hqnudge), `StampDone`, and
  `dispatch.ClearAwaited(pane)`. Non-awaited panes keep the existing DoneDue switch.
- [x] 3.3 The `gone` path also `dispatch.ClearAwaited(pane)` (no stale marker).

## 4. Tests

- [x] 4.1 `deferDone` truth table: awaited+attended → false (fires); awaited+unattended
  → false; !awaited+attended (unattended mode) → true (tally); !awaited+unattended →
  false; DoneTick → true unless awaited; DoneAlways → false.
- [x] 4.2 Registry round-trip + gone-clears-it (if the gone path is unit-reachable).

## 5. Gate / spec

- [x] 5.1 `make check` green.
- [x] 5.2 `openspec validate done-wake-keyed-on-awaited --strict` passes.
- [x] 5.3 Fold deltas into `hq-wake-protocol` + `agent-dispatch` specs and archive.
