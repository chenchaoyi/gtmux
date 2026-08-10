# Tasks — standing-wake-backoff

Status: IN PROGRESS. Alarm-channel change — the risky direction is silent suppression of
a real alarm; every slice pairs a "quiet when unchanged" test with a "resumes on change"
test.

Slice 1 landed: audits §1 + the `stuck·waiting` premise gate §6.

## 1. Audits first (settle spec-vs-implementation before adding mechanism) — DONE

- [x] 1.1 `resource·warn` identical-payload ×4-in-40-min (2026-08-04). **VERDICT: the
      incident serve predated the shipped gate; no hole to fix, form 1 needs no new
      mechanism.** `tierGate` shipped 2026-07-18 (#496), hysteresis 2026-08-01 (#653),
      both before the incident; the resident launchd serve keeps its binary until
      restarted. Proven rather than argued —
      `TestAudit_ResourceWarnIdenticalRepeatsAreUnreachable` drives the real `tierStep`
      at shipped defaults: a steady amber knocks **once** in 40 min, and even a tier
      dithering as fast as `ConfirmSamples` allows caps at 2, so 4 is unreachable.
      → **Change narrowed:** drop form 1; §4 keeps only delivery revalidation + hint
      decoupling (a DIFFERENT failure — the premise dying between decision and delivery).
- [x] 1.2 `usage·warn burn` ×3-in-one-round (2026-08-05). **VERDICT: not a dedup bug — a
      missing anti-flap layer.** `Evaluate` reports the FIRST breached layer, ctx before
      burn, so a session whose ctx dithers around its threshold alternates ctx → burn →
      ctx → burn; every alternation is a layer CHANGE, which is exactly what `watchUsage`
      treats as new. The burn breach never left; only its turn to speak did. Reproduced in
      `TestAudit_UsageBurnRepeatsWhenCtxDithers` (3 knocks from one steady 23.6M breach,
      1 knock when ctx holds still — which is why every test written for the dedup passed).
      `resource·warn` pays for this with hysteresis + confirm-samples + restate interval;
      `usage·warn` has none of the three. → **§5 gains that**, alongside form 2
      (`TestAudit_BurnTotalHasNoDeAssertCondition`: a monotonic total has no exit).

## 2. Family rule (hq-wake-protocol ADDED requirement)

- [ ] 2.1 Add a delivery-side revalidation seam in `internal/hqwake`/`internal/hqnudge`:
      a queued standing-class wake can carry a premise probe re-run at flush; probe
      says premise gone ⇒ drop (and record a control trace), changed ⇒ re-render.
- [ ] 2.2 Unchanged-world suppression: persist per-class last-knock fingerprint
      (breach set + payload hash + fleet/turn cursors); an identical fingerprint
      suppresses the repeat, any drift re-arms. Periodic safety floor stays.
- [ ] 2.3 Tests: premise-gone drops; premise-changed re-renders; unchanged fingerprint
      suppresses; a single new fleet event or HQ turn re-arms.

## 3. self-rotate backoff + age gate (C17)

- [ ] 3.1 `selfRotateDecide` takes the world-change inputs (non-HQ events since last
      knock, HQ-pane turns since last knock, current ctx vs threshold); an age-only
      breach with ctx recovered and a static world holds instead of knocking.
- [ ] 3.2 Unchanged-breach repeats suppress-until-change (replacing the flat 1800 s
      re-knock); rotation (session id change) still clears everything.
- [ ] 3.3 Tests pinned to the ledger shapes: the 17-knock night collapses to 1; the
      post-compaction "ctx recovered but age keeps knocking" case goes quiet; a new
      fleet event or a genuine ctx re-breach knocks again.

## 4. resource·warn revalidation + hint decoupling (C15 forms 1 & 3)

- [ ] 4.1 Pre-delivery re-sample: the tier claimed by a queued `resource·warn` is
      re-read at flush; improved-through-tier ⇒ not delivered.
- [ ] 4.2 Decouple the reclaim hint: the alarm line stands alone; the hint (measured
      false-positive ×6 on the simulator suggestion) is marked advisory and dropped
      when its probe fails, without dropping the alarm.
- [ ] 4.3 Tests: improving metrics deliver no repeat; a genuine tier escalation still
      always delivers (never suppressed by any rule here).

## 5. usage thresholds: monotonic + reset-aware (C15 forms 2 & 3)

- [ ] 5.1 `burn` (and any cumulative) layers alarm on rate / windowed delta only;
      remove/convert total-over-line checks (config migration note for usage.json).
- [ ] 5.2 ctx-based warns re-sample at delivery (auto-compact voids them in flight).
- [ ] 5.3 Tests: a growing total under a flat rate never alarms; a rate spike does; a
      warn queued before an auto-compact does not deliver after it.

## 6. stuck·waiting ask gate (C20③)

- [x] 6.1 The lifecycle watchdog checks the wait's PROVENANCE at escalation time. **The
      criterion changed during implementation and the change is deliberate:** the proposal
      said "empty ask ⇒ no escalation", but `ask` is populated only from a parseable
      replyable MENU, so "no ask" is equally the state of an agent blocked on a FREE-FORM
      question — a genuine stuck wait, and silencing it trades the C20③ harm for the C23
      one (a session idled four hours because nothing announced it). The marker already
      records provenance: hook-written kinds (permission/plan/question) mean the agent
      asked, menu or not; `startup`/`draft` are gtmux's own screen verdict. Gated on
      `hook.IsAskKind` — the predicate #755 added for the approval card, same reason.
      The episode marker still does not burn: `shouldEscalate` returns false before
      `state.Touch`, so a later real ask in the same episode escalates once.
- [x] 6.2 `TestShouldEscalate_PremiseIsProvenanceNotAskText` pins BOTH directions,
      including the one the literal criterion would have lost: kind `question` (often
      free-form prose, no menu) MUST still escalate.
- [ ] 6.3 Cross-reference: the false `waiting` itself (codex render latency) is the
      C3/C20①② family, tracked with `mobile-send-receipt-first` — not fixed here.

## 7. Consistency (per the repo rule)

- [ ] 7.1 Sync deltas into `openspec/specs/{hq-wake-protocol,resource-watch,usage-watch,supervisor-agent}/spec.md`.
- [ ] 7.2 `docs/cli.md` wake-class table: note the standing-class revalidation semantics
      where user-visible; CLAUDE.md self-rotate paragraph if wording drifts.
- [ ] 7.3 If the playbook text changes (board/KB-current half of the age gate), bump
      `hqPlaybookVersion`.
- [ ] 7.4 Archive this change once implemented; HQ moves ledger C17/C15/C20③ accordingly.
