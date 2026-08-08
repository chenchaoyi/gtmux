# Tasks — hq-unread-noise

Status: COMPLETE — shipped in #728, archived. Perception-layer change — implement behind review, never
direct-to-main; a wrong exclusion silently blinds the completeness net.

## 1. Echo audit (settles the C14 residual before touching mechanism) — DONE

Results: `audit-echo-2026-08-08.md`.

- [x] 1.1 Determine whether the 2026-08-04 self-excitation loop (cursor 8439→8450) ran on
      a pre-#650 resident serve. **Verdict: NO** — the first `unread` knock in the stream
      is seq 7039 / 2026-08-02, two days earlier, so the sensor and its pane filter were
      live. Residual narrowed to double-knock + composition, as the proposal predicted.
- [x] 1.2 Measure what fraction of the debt had already been delivered in a driver-ack'd
      wake batch. The stream cannot answer it directly (a delivery records only its batch
      id), so the audit measured the two quantities that bound it: class-wake ELIGIBILITY
      of the debt (78.2 % upper bound, 21.8 % claimed by no class) and the actual echo —
      **68.7 % of the records a knock's pull returns are HQ's own** (75.1 % for the ≤2 case).
      HQ's ~70 % estimate is confirmed as a number and re-attributed to the PULL.
- [x] 1.3 Verdict on the open question: **an ack'd-wake-delivered event is NOT
      consumption-equivalent.** Four reasons recorded in the audit §1.3 and summarised in
      the proposal — decisively, pane-less records can never be delivered as a class wake
      (`hook.go:679`), so "delivered ⇒ read" would blind the one population with no other
      channel.

## 2. Pane-less lifecycle BLINKS stop counting (C7 — criterion corrected by 1.x)

The audit overrode this task's original wording ("skip records with an empty `pane`"),
which would have stopped counting 39 real pane-less agent turns and the 9 `gtmux:*`
maintenance triggers #647 shipped to reach HQ. Adopted rule: liveness pairing.

- [x] 2.1 `internal/hq/unread.go` `unreadScan` (was `unreadCount` — it now returns a tally, not a bare count): skip a pane-less `SessionStart` when a
      pane-less `SessionEnd` from the same agent lands within `unreadBlinkPairSec` (10 s),
      and skip that end with it. Keep them in `maxSeq` advancement and in every stream read.
- [x] 2.2 Test pinned to the measured shape: same-second `SessionStart`/`SessionEnd` pairs
      with empty pane (seq 8472–8475 form) produce zero debt; the same records still appear
      in `events.ReadSince`.
- [x] 2.3 Anti-regression tests for the three populations the blunt rule would have eaten:
      a pane-less agent TURN counts, a `gtmux:*` control record counts (extend the existing
      `TestUnreadCountsControlRecords`), and an UNPAIRED pane-less `SessionStart` (a native
      session coming online — 13/28 of them) counts.

## 3. Knock-line composition (diagnosability)

- [x] 3.1 `unreadScan` returns a compact by-pane/kind tally; `unreadSensorFor` renders it
      into the `unread` line (`3 unconsumed (%21 ×2 · control ×1)` form), bounded so a
      wide fleet cannot bloat the line past the wake-line cap.
- [x] 3.2 Fixture test for the line format (the wake-line grammar is pinned by fixtures —
      extend, don't fork).

## 4. Loud non-counting read (B9 tool side)

- [x] 4.1 `internal/hq/eventscmd.go`: an unfiltered `--since-seq` read from a cwd strictly
      inside the HQ home (subdir ≠ home) that does not advance the watermark warns on
      stderr, naming the home to run from; exit code unchanged.
- [x] 4.2 Whether to ALSO key on `$TMUX_PANE == HQ pane`: **NO** — it would put tmux
      resolution on a path that today touches no tmux at all, and a wedged tmux hanging the
      pull HQ makes on every wake is the exact failure mode that froze the radar once
      already. The measured B9 evidence is 5 for 5 INSIDE the home, so the cwd rule covers
      every observed case; a cwd fully outside the home is indistinguishable from a
      bystander's read, which must stay silent. Verdict recorded in `insideHQHome`'s doc
      comment, where the next reader will meet it.
- [x] 4.3 Tests: subdir read warns and does not consume; HQ-home read consumes silently;
      an unrelated-cwd read neither warns nor consumes; `--ack` behavior unchanged.

## 5. Consistency (per the repo rule)

- [x] 5.1 Sync this change's delta into `openspec/specs/hq-wake-protocol/spec.md`.
- [x] 5.2 Update the `docs/cli.md` `gtmux events` section (warning behavior) and the
      CLAUDE.md watermark paragraph if wording drifts.
- [x] 5.3 Archived after #728 merged. HQ still owes its own side: move ledger C7 / C14 /
      B9 (tool half) and the verified-fixed B3 to 已关闭 in its notes — that is HQ's
      knowledge base, not this repo.

## 6. The pull shows the debt (audit-driven — folded in on the commander's call)

- [x] 6.1 `internal/hq/eventscmd.go` `pullView`: an unfiltered `--since-seq` read run from
      the HQ home omits what `unreadScan` omits — the caller's own pane records and the
      paired blinks — and reports the withheld count on stderr. `--all` restores the raw
      view. Both still consume: neither shows LESS than the debt, which is the property a
      `--severity` read lacks. A non-supervisor read is returned unchanged.
- [x] 6.2 The caller's own pane comes from `$TMUX_PANE`, never from resolving the HQ pane
      through tmux — the read path stays free of the round-trips whose wedging froze the
      radar once. (Consistent with 4.2: knowing your OWN pane is free; knowing whether you
      are the SUPERVISOR is what would have cost a tmux call.)
- [x] 6.3 Tests: the pull hides HQ's echo + blinks, prints only the debt, names the withheld
      count, and still advances the watermark; `--all` shows everything, says nothing, and
      also consumes; a bystander's read is unreshaped and silent.
- [x] 6.4 Playbook v17 (`hqPlaybookVersion` bumped, per the repo rule) teaches the new pull
      shape + `--all`, and the cwd rule behind the B9 warning.
