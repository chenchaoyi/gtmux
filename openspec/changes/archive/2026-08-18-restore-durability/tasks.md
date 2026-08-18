# Tasks — restore-durability

Status: IMPLEMENTED (2026-08-18, one pass; every slice's tests landed with it, and each
was verified to go RED with its fix backed out).

## 1. The backstop saves on evidence, not configuration

- [x] 1.1 `shouldBackstopSave(statusRight, lastPath, now)` becomes a staleness question;
      `backstopStaleAfter` gives an armed autosaver a wider (but finite) grace; the
      trigger check survives only as the input to that choice.
- [x] 1.2 `maybeBackstopSave` yields `backstopArmedYield` and RE-CHECKS before saving
      when a trigger is present — the wake-from-sleep collision — and logs both the
      stand-down and the save.
- [x] 1.3 `gtmux doctor`'s `resurrect autosave` row reports the save's real age: an armed
      trigger idle past the armed threshold is flagged, not called OK.
- [x] 1.4 Tests: the bug in its own terms (trigger present + 6h-old save MUST fire);
      the race guard (a save that keeps landing stands the backstop down, armed or not);
      the two thresholds; no save at all. Verified red against the old rule.

## 2. Pane-keyed state is reconciled with the live panes

- [x] 2.1 `state.ReapDeadPaneState(live)` — every pane-keyed family (13 plain dirs, the
      `.json` sends dir, and hqwake's three prefixed families), gated on `IsPaneID` and
      refusing an empty live set. `resume`/`usage`/`native` and hqwake's bookkeeping
      files deliberately excluded.
- [x] 2.2 `tmux.LivePaneIDs()`; `hq.deadPaneSweep` on the serve slow tick (5-min gate)
      and `hq.ReapDeadPaneStateNow()` from restore's `afterRestore`, before the
      conversations come back.
- [x] 2.3 `dispatch.Task.StaleUndelivered(now)` — a two-week-old undelivered dispatch
      keeps its ledger truth and loses its authority over a pane id;
      `radar.StuckDispatchKind` honours it.
- [x] 2.4 `prompt.IsStartupPicker(capture, agent)` — reopened-session chrome, keyed by
      agent through the registry, with IsStartupGate's region + faint narrowings;
      `classifyStuck` returns "" for it (NOT "startup" — a picker is not a block);
      `looksLikeStartupChooser` folds onto it.
- [x] 2.5 Tests: every family swept / non-pane-keyed state untouched / empty live set
      refused / `IsPaneID`; the sweep's throttle; the resume menu is neither a draft nor
      a gate, and menu wording in scrollback still leaves a real draft readable; the
      stale-dispatch predicate both ways.

## 3. What came back is checked against what was saved

- [x] 3.1 `normalizeLayout` — drop the checksum and the leaf pane numbers, so the same
      arrangement compares equal after a server restart renumbered its panes.
- [x] 3.2 `savedShapes` (window lines + per-window pane counts, layout located by SHAPE
      so a shifted line can't fool it) and `liveShapes`; `layoutDrift` pure comparison,
      pane count first; `reportLayoutDrift` to stderr (bounded) + `restore.log` (full).
- [x] 3.3 `saveAgeNote` printed on every restore — which moment is coming back and how
      old it is.
- [x] 3.4 Tests: renumbered panes compare equal, a real rearrangement does not; the
      extra-pane case in its production shape; a faithful restore is silent; a
      post-save session is not reported; a missing window is; the age note.

## 4. Docs, specs, gates (same PR)

- [x] 4.1 `docs/cli.md`'s restore section: save freshness + the backstop, the layout
      check, pane-record reconciliation.
- [x] 4.2 Spec deltas: session-restore MODIFIED (the backstop's trigger) + ADDED (the
      layout check, the age note, pane-record reconciliation); agent-radar MODIFIED (the
      stuck-dispatch classifier).
- [x] 4.3 Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
      `scripts/check-design.sh`, `openspec validate --specs --strict`.
- [x] 4.4 Verified read-only on the live machine: 22 saved vs 22 live windows parse and
      normalize identically (no false drift); a doctored save reproduces the drift report
      end-to-end; the sweep reaps exactly the dead-pane records and leaves HQ's watermark
      and a live pane's records alone.
