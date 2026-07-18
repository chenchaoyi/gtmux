# Tasks: draft-detect-excludes-ghost-suggestion

## 1. Faint-aware primitive

- [x] 1.1 `internal/dispatch`: `stripAnsiDroppingFaint(s string) string` — one pass:
  parse SGR (`2`→faint on, `0`/`22`→off), drop other CSI/OSC escapes, emit literal
  runes only when faint is off.
- [x] 1.2 `internal/dispatch`: `DraftOfColored(coloredCapture string) (draft string,
  structured bool)` = `SplitInputRegion(stripAnsiDroppingFaint(coloredCapture))` draft.
- [x] 1.3 Unit tests: a faint ghost span drops to empty; a bright draft survives; mixed
  (bright input + a faint tail suggestion) keeps only the bright part; borders survive
  so `structured` stays true; a plain (no-ANSI) capture is unchanged (identity).

## 2. Color capture

- [x] 2.1 `internal/tmux`: `CaptureFullColor(pane string) string` = `capture-pane -e -p
  -S -200` (color twin of `CaptureFull`).

## 3. Apply to the three "any user draft?" sites

- [x] 3.1 `agents.go` `stuckDispatchKind`: keep plain `CaptureFull` for `IsStartupGate`;
  use `dispatch.DraftOfColored(tmux.CaptureFullColor(paneID))` for the draft check.
- [x] 3.2 `nudge.go` `wakeDone` guard: same swap for its draft check.
- [x] 3.3 `internal/hqnudge`: add an injectable `captureColor` (default
  `tmux.CaptureFullColor`) and use `dispatch.DraftOfColored` in the draft-guard
  (`Deliver`/`Drain`/`Pending` paths). Leave the wake-ack `SplitInputRegion` (specific
  `#id` match) and `deliver.go` (specific payload match) UNCHANGED.

## 4. Tests

- [x] 4.1 `dispatch`: the `stripAnsiDroppingFaint` / `DraftOfColored` cases (1.3).
- [x] 4.2 `hqnudge`: a faint-ghost HQ composer does NOT hold a nudge (delivers), while a
  bright half-typed draft still queues.
- [x] 4.3 (If cheaply testable) an `app`/`hook` test that `stuckDispatchKind` /
  `wakeDone` no longer flag a faint-ghost composer — or rely on the dispatch-level unit
  test if the pane capture is not injectable there.

## 5. Gate / spec

- [x] 5.1 `make check` green.
- [x] 5.2 `openspec validate draft-detect-excludes-ghost-suggestion --strict` passes.
- [x] 5.3 Fold deltas into the three specs and archive the change.
