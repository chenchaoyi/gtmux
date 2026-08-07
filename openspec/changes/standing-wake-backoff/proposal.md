# standing-wake-backoff — a repeating knock re-checks its premise and backs off when nothing changed

Origin: HQ's charter-flags ledger (`~/.config/gtmux/hq/knowledge/charter-flags.md`),
entries **C17 · C15 · C20③**, batched on the commander's 2026-08-07 order. Companions:
`hq-unread-noise` (counting correctness), `hq-signal-ergonomics` (presentation).
Proposal only — the wake layer is HQ's hearing; it changes via review.

## The family criterion (the ledger already named it; this change makes it a spec rule)

Several wake classes repeat until an act clears them — `self-rotate` until the session
id changes, `unread` until consumption, `resource·warn`/`usage·warn` on their tiers,
`stuck·waiting` on an unresolved wait. The repeat-until-cleared shape is correct (a
knock is a debt, not an FYI). What the measured incidents share is that the REPEAT
fires without re-asking two questions the first knock answered:

1. **Does the premise still hold?** The world the alarm described can change while the
   alarm is queued or between repeats — an auto-compaction clears the ctx it warned
   about, a metric improves back through its tier — and the knock arrives asserting a
   state that no longer exists.
2. **Has anything changed since the last knock?** A repeat that carries the identical
   breach set, identical payload, against an unchanged fleet, tells the consumer
   nothing the previous knock didn't — while each delivery costs HQ a full turn (and,
   for `self-rotate`, that turn burns the very context the knock warns about).

And beneath both: **a knock must be consumable** — before repeating, ask whether the
knocked role can actually perform the act that clears the debt. (C17's original form:
`self-rotate` was un-actionable by HQ until playbook v16 / `gtmux hq --rotate` made
rotation HQ's own verb; the capability now exists, but the principle generalizes and
belongs in the spec.)

## Why — the measured incidents

### C17 — `self-rotate` has no backoff, and age alone re-fires forever (confirmed at HEAD)

`selfRotateDecide` (`internal/hq/selfrotate.go`) re-knocks every `SelfRotateRepeatSec`
(1800 s) while ANY criterion stands, with no change detection. ctx is re-sensed each
tick, so a recovered ctx clears that criterion — but **age can never recover**, so an
age-only breach re-knocks unconditionally until rotation.

Ledger evidence: 2026-08-03 measured 5+ knocks; **2026-08-05 one night ~17 knocks
across 10 hours** (ctx 78%→96%, age 8 h→18 h), with the fleet state and the handover
section **completely unchanged from the 2nd knock on** (0 needs-you, zero non-HQ events
in the window). The sharpest observation: **ctx was pushed from 78% to 96% largely by
the answer-the-knock turns themselves — the number it warns about is partly its own
product**. Backoff alone would have collapsed those 17 turns into 1. The closing
observation (2026-08-05 daytime): the harness auto-compacted ctx 98%→21%, **the
triggering condition disappeared, and the knocking continued** — now hanging on
`age 24h ≥ 12h`. Criterion: **when the board and KB are current, the fleet is static,
and ctx has recovered, age SHALL NOT independently re-fire** — a healthy session must
not be nagged indefinitely. gtmux can sense the mechanical half of that (ctx below
threshold ∧ no non-HQ fleet events ∧ no HQ-pane turns since the last knock); "board/KB
current" is HQ-side and stays in the playbook.

### C15 — `resource·warn`/`usage·warn`: three escalating forms, all premise failures

1. **Identical-payload repeats while metrics improved.** 2026-08-04: 4 knocks in
   40 minutes, payload byte-identical (`memory warn — reclaim: iOS Simulator runtime
   (2 procs)`), while mem_free went 46%→49% and disk 47→54 GB. False-alarm family
   count across two HQ incumbents: ×5 (a later instance with mem_free 49% / load 0.32 /
   disk 51 GB, no deterioration, payload still byte-identical). NOTE: the by-tier
   dedup + hysteresis + 30-min minimum-restate ARE already spec'd (`resource-watch`,
   "Tick-driven warnings with correct dedup", #475 2026-07-17) — the measured behavior
   violates the spec, so the first task is an implementation-vs-spec audit (it is
   **uncertain** whether the incident serve predated the shipped dedup; the resident
   serve keeps old code until restarted). The reclaim hint's coupling is real either
   way: the simulator suggestion was measured as a false positive ×6 yet is the only
   information the line carries — decouple the hint from the alarm.
2. **A monotonic cumulative can never clear its alarm.** 2026-08-05: `usage·warn burn`
   knocked three times in one round (`%18` 23.6M→23.7M→23.9M). `burn` only ever grows;
   a total-over-line threshold on it has no de-assert condition in physics. Criterion:
   **cumulative quantities SHALL alarm on rate or windowed delta, never on total
   crossing a line.**
3. **Even a rate criterion fails if the value resets mid-flight (the key one).**
   2026-08-05: `usage·warn … ctx→80% in ~4m` — already the rate form demanded above —
   yet when HQ cross-checked the pane's status bar it read `68.5K 34%`: the harness's
   auto-compact had zeroed the number **while the warning was in flight**. Conclusion
   from the ledger, adopted here: the failure source is not threshold-vs-rate design
   but that `ctx` has an external reset event (auto-compact) neither gtmux nor HQ
   controls, so **delivery-side revalidation is the only fix that holds**: gtmux
   re-samples the premise immediately before delivering the wake and drops/requeues it
   when the premise no longer holds; the consumer-side rule (HQ re-reads the live value
   before printing) already exists in the playbook and caught this instance.

### C20③ — an ask-less `waiting` escalates to `stuck·waiting` (premise never re-checked)

2026-08-06, `%31` (codex): composer held only a dim placeholder, digest showed
`ask: None`, `err: None`, the target had long since answered — yet radar still reported
`waiting` and the lifecycle watchdog escalated `stuck·waiting` after 10 minutes
(knocked ×2). Root cause of the false `waiting` itself is the codex >1 s render latency
family (C3②/C20①② — tracked with `mobile-send-receipt-first` and the dispatch line, out
of scope here). In scope: the escalation's premise check. **A `waiting` with an empty
`ask` SHALL NOT escalate to `stuck·waiting`** — no question is waiting on anyone, so
nobody is stuck. The watchdog re-reads the ask at escalation time; dim-only composer
text is a placeholder, not a draft.

## What changes

1. **A family-level spec rule** (`hq-wake-protocol`, ADDED): every repeat-until-cleared
   wake class SHALL (a) re-validate its premise immediately before each delivery and
   not deliver when the premise no longer holds; (b) suppress a repeat whose breach set
   AND payload are unchanged since the last delivered knock while the observed world
   (fleet events, HQ turns) is also unchanged — re-arming on any change; (c) name, in
   its class definition, the act that clears it and who can perform it.
2. **`self-rotate` gains the two checks** (`hq-wake-protocol`, MODIFIED): age never
   independently re-fires against a static world with recovered ctx; unchanged-breach
   repeats back off (suppress-until-change; the periodic re-check cadence stays as the
   safety floor).
3. **`resource·warn` delivery revalidates; reclaim hint decoupled** (`resource-watch`,
   MODIFIED): pre-delivery re-sample (a queued warn whose re-sampled tier differs from
   the claim is not delivered as-is); the reclaim suggestion becomes clearly advisory
   and its absence never blocks the alarm. Plus an implementation-vs-spec audit task
   for the measured identical-payload repeats.
4. **`usage` thresholds respect monotonicity and resets** (`usage-watch`, MODIFIED):
   cumulative quantities alarm on rate/windowed delta only; ctx-based warns are
   re-sampled at delivery because auto-compact can void them in flight.
5. **`stuck·waiting` requires a non-empty ask** (`supervisor-agent`, MODIFIED watchdog
   requirement).

## Impact

- Specs: `hq-wake-protocol`, `resource-watch`, `usage-watch`, `supervisor-agent`.
- Code (when implemented): `internal/hq/selfrotate.go`, `internal/hq/watchdog.go`,
  `internal/hqwake` (delivery-side revalidation seam), resource/usage warn paths.
- Risk: suppression logic on an alarm channel can hide a real alarm. Every suppression
  added here re-arms on change and keeps a periodic safety floor; tests must cover the
  "world changed → knock resumes" direction as heavily as the "unchanged → quiet" one.
- Numbers kept as arguments: 17 knocks/10 h → 1 with backoff; ctx 78%→96% self-inflicted;
  4 identical payloads in 40 min against improving metrics; burn ×3 in one round;
  `ctx→80% in ~4m` warn vs live `34%`.
