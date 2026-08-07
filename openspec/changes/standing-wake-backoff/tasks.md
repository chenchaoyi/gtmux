# Tasks — standing-wake-backoff

Status: PROPOSED (not started). Alarm-channel change — the risky direction is silent
suppression of a real alarm; every slice pairs a "quiet when unchanged" test with a
"resumes on change" test.

## 1. Audits first (settle spec-vs-implementation before adding mechanism)

- [ ] 1.1 `resource·warn` identical-payload ×4-in-40-min (2026-08-04): determine whether
      the incident serve predated the shipped by-tier dedup/hysteresis/min-restate
      (#475). If the spec'd dedup was running, find and fix the hole; if not, record
      that the spec already covers form 1 and narrow this change to forms 2–3 +
      delivery revalidation.
- [ ] 1.2 `usage·warn burn` ×3-in-one-round (2026-08-05): reproduce against the current
      dedup key (session+layer) and identify why an unchanged breach re-knocked.

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

- [ ] 6.1 The lifecycle watchdog re-reads the ask at escalation time; empty ask (or
      dim-only composer text) ⇒ no `stuck·waiting` escalation, and the episode marker
      does not burn (a later real ask in the same episode may still escalate once).
- [ ] 6.2 Test pinned to the `%31` shape: waiting + `ask: None` + dim placeholder ⇒ no
      escalation; same episode gaining a real ask ⇒ one escalation.
- [ ] 6.3 Cross-reference: the false `waiting` itself (codex render latency) is the
      C3/C20①② family, tracked with `mobile-send-receipt-first` — not fixed here.

## 7. Consistency (per the repo rule)

- [ ] 7.1 Sync deltas into `openspec/specs/{hq-wake-protocol,resource-watch,usage-watch,supervisor-agent}/spec.md`.
- [ ] 7.2 `docs/cli.md` wake-class table: note the standing-class revalidation semantics
      where user-visible; CLAUDE.md self-rotate paragraph if wording drifts.
- [ ] 7.3 If the playbook text changes (board/KB-current half of the age gate), bump
      `hqPlaybookVersion`.
- [ ] 7.4 Archive this change once implemented; HQ moves ledger C17/C15/C20③ accordingly.
