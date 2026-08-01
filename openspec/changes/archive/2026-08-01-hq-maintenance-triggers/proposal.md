# hq-maintenance-triggers — make the periodic distill / self-check actually land

## Why

`hq-capture-loop` deferred the distill auto-triggers behind an **observation gate**: ship
the protocol + `gtmux capture`, run distillation manually for a while, and only mechanize
the firing if capture still slipped. The 2026-07-19 note recorded the call as *"先让手动
distill 跑一阵,滑了再上"*.

The observation is in, and it falsifies the premise the gate rested on. Over the 13 days
2026-07-19 → 2026-08-01:

- **0** distillation passes actually ran. The `pending-distill` spool is empty — the
  capture path has never been drained, because it has never been read.
- `gtmux events` (6 625 records) contains **not one** `gtmux:distill` or
  `gtmux:self-check` line.

But the sensors were never the problem — they fired correctly the whole time. The live
`hq-feed/spool.jsonl` holds **2** `gtmux:distill` records (2026-07-18 and 2026-07-25, both
`weekly`) and **24** `gtmux:self-check` records (`daily`/`idle`). The serve slow-tick did
its job every time.

**The root cause is DELIVERY, not cadence.** `hqfeed.EmitControl` writes the trigger to
the feed SPOOL and nowhere else, and both readers of the spool are unreachable:

1. **Nothing knocks.** Under hq-perception-v2 the ONLY arousal is a signal line typed into
   the HQ pane, and the seeded playbook says of the spool daemon: *"You do NOT need to
   tail it — wake lines knock, and you pull deltas."* `feed-degraded` and `wake-degraded`
   deliver a wake line beside their control record; `distill` and `self-check` deliver
   nothing. They were spec'd as *"delivered on the silent feed … NOT a typed wake line"* —
   which, given a pure pull-side HQ with no timer of its own, means delivered to nobody.
2. **Nothing is auditable.** `gtmux events` reads only the session-event journal, never
   the spool, so a control record leaves no trace in the one stream a human or HQ
   actually queries. That is why the trigger looked like it had never run at all.

The deferral therefore tested a mechanism that was never wired to a consumer. It is not
evidence that the periodic ritual is unnecessary; it is evidence that **an event-driven
agent with no clock cannot be relied on to self-initiate anything.** Distilling is HQ's
"single most important job" per its own charter, and it has run zero times in two weeks.

## What changes

Three legs, all on the **serve/feed side** — a resident process raises and DELIVERS the
trigger; HQ is never asked to remember. Cadence is unchanged and deliberately coarse.

- **① Delivery — the trigger knocks.** Two new wake classes, `distill` and `self-check`,
  at **`PriorityStanding`** (the lowest): they queue behind every blocked agent, are the
  first evicted at the queue cap (they re-fire on their own cadence), and ride the same
  draft-guarded, acked `hqnudge` channel as every other wake. Budget in a quiet fleet:
  **≈ 1 line/week (distill) + ≈ 1 line/day (self-check)**. This overturns the
  `NOT a typed wake line` clause of the previous spec, keeping its *intent* (never
  interrupt decision traffic) via the priority floor rather than via silence.
- **② Audit — the trigger is in `gtmux events`.** Maintenance triggers are appended to the
  session-event JOURNAL (`events.Append`) instead of being hand-written to the spool. The
  feed daemon then spools them on its normal path, so `hq-feed --tail` renders
  `[CONTROL gtmux:distill]` exactly as before — but now `gtmux events --since-seq` carries
  them too, `events.Format` renders a control record legibly, and
  `gtmux events --since 30d | grep distill` answers *"did it run?"* directly.
  - A **self-feed guard** comes with it: `events.IsControl` excludes gtmux's own control
    records from the sensors' own inputs. Without it the first journal-visible
    `gtmux:distill` would permanently defeat distill's zero-change gate, and a
    `gtmux:self-check` record would read as "the user was recently pinged" and kill
    self-check's idle trigger. gtmux's own bookkeeping is not fleet activity.
- **③ Observability — an at-a-glance overdue verdict.** `gtmux doctor` grows an **HQ
  maintenance** section (present only when an HQ home exists) showing when each pass last
  ran and whether it has SLIPPED past its floor plus a grace window. `gtmux capture
  --list` gains a one-line header with the same last-distill age, since that queue is what
  a distill pass drains.

Cadence stays as `hq-knowledge-distillation` decided it — no new noise:

| trigger | rate limit | fires on |
|---|---|---|
| `distill` | ≤ 1 / day | WEEKLY floor · EVENT-VOLUME floor (5 000) · **pending-distill SPOOL ≥ 5** (new), all behind a ZERO-CHANGE gate |
| `self-check` | ≤ 1 / hour | DAILY floor · IDLE (12 h quiet) · THRESHOLD (ledger/journal/cursor) |

The **spool floor** is the one cadence addition: it is layer ③(c) of `hq-capture-loop`,
already spec'd with a default of 5, and it is what makes a *captured* lesson reach the KB
in days rather than waiting out the week. The density-K and correction-class triggers stay
deferred — neither has a `correction`-class event to key on yet.

## Impact

- **Affected specs:** `supervisor-agent` (the distill trigger's delivery + the spool floor
  + both playbook rituals + the audit/observability requirements) and `hq-wake-protocol`
  (the class list + a standing-priority maintenance requirement). The doctor section is
  spec'd under `supervisor-agent` because it reports HQ behavior — `env-doctor` owns the
  check framework, not what a check means.
- **Playbook version:** `hqPlaybookVersion` 11 → 12 — the wake-class list must teach
  `distill` / `self-check`, and both rituals now say the record ALSO lands in
  `gtmux events` (so a missed knock is recoverable by pull, not lost).
- **No new CLI command.** `gtmux doctor` and `gtmux capture --list` gain output only.
- **Reuse, don't rebuild:** the sensors, watermark, control-record names, spool rendering
  and wake queue all already exist. This change re-points their OUTPUT at a consumer.

## Non-goals

- Not a cadence loosening. The floors, the rate limits and the zero-change gate are
  untouched; a quiet fleet still costs zero wakes.
- Not the density-K or `correction`-class distill triggers (still deferred).
- Not events-log retention, and not the feed/wake degradation paths — `feed-degraded` and
  `wake-degraded` keep emitting straight to the spool, which is correct precisely because
  the journal→spool path is what may be broken when they fire.
