# Tasks — hq-unread-noise

Status: PROPOSED (not started). Perception-layer change — implement behind review, never
direct-to-main; a wrong exclusion silently blinds the completeness net.

## 1. Echo audit (settles the C14 residual before touching mechanism)

- [ ] 1.1 Determine whether the 2026-08-04 self-excitation loop (cursor 8439→8450) ran on
      a pre-#650 resident serve: correlate the machine's serve start time / binary
      version against the v0.44.10 install date. If yes, record it in the proposal and
      narrow the residual to double-knock + composition only.
- [ ] 1.2 Measure, over a representative window of the live stream, what fraction of
      unread-debt events had already been delivered inside a driver-ack'd wake batch
      (HQ's 2026-08-07 estimate: ~70% of wakes are echoes; get the instrumented number).
- [ ] 1.3 With 1.2's number, decide the open question: does an ack'd-wake-delivered event
      count as consumption-equivalent for the debt? Write the verdict + reasoning back
      into this change before implementing anything that depends on it.

## 2. Empty-pane session events stop counting (C7)

- [ ] 2.1 `internal/hq/unread.go` `unreadCount`: skip records with an empty `pane` when
      tallying; keep them in `maxSeq` advancement and in every stream read.
- [ ] 2.2 Test pinned to the measured shape: same-second `SessionStart`/`SessionEnd`
      pairs with empty pane/session (seq 8472–8475 form) produce zero debt; the same
      records still appear in `events.ReadSince`.

## 3. Knock-line composition (diagnosability)

- [ ] 3.1 `unreadCount` returns a compact by-pane/kind tally; `unreadSensorFor` renders it
      into the `unread` line (`3 unconsumed (%21 ×2 · control ×1)` form), bounded so a
      wide fleet cannot bloat the line past the wake-line cap.
- [ ] 3.2 Fixture test for the line format (the wake-line grammar is pinned by fixtures —
      extend, don't fork).

## 4. Loud non-counting read (B9 tool side)

- [ ] 4.1 `internal/hq/eventscmd.go`: an unfiltered `--since-seq` read from a cwd strictly
      inside the HQ home (subdir ≠ home) that does not advance the watermark warns on
      stderr, naming the home to run from; exit code unchanged.
- [ ] 4.2 Decide (implementation-time) whether to also key on `$TMUX_PANE == HQ pane`;
      document the verdict either way.
- [ ] 4.3 Tests: subdir read warns and does not consume; HQ-home read consumes silently;
      an unrelated-cwd read neither warns nor consumes; `--ack` behavior unchanged.

## 5. Consistency (per the repo rule)

- [ ] 5.1 Sync this change's delta into `openspec/specs/hq-wake-protocol/spec.md`.
- [ ] 5.2 Update the `docs/cli.md` `gtmux events` section (warning behavior) and the
      CLAUDE.md watermark paragraph if wording drifts.
- [ ] 5.3 Archive this change once implemented; HQ moves ledger C7/C14/B9(tool half) and
      the verified-fixed B3 to 已关闭 in its own notes (not this repo).
