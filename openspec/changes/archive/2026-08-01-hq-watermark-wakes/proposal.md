# hq-watermark-wakes — perception by watermark, not by whitelist

## Why

On 2026-08-01 at 12:29:57 the pane `%16` finished an install the commander was waiting on.
The turn-end went into the event stream complete and correct: seq 6665, `Stop`, notable
severity, summary intact. **HQ was never told.** Three minutes later the commander asked by
hand: *"%16 已经完成了,似乎你没接收到?"*

The data was there. The delivery never happened.

Three layers deep, only the third is the cause:

1. **Surface.** The `done` wake fires for a completion gtmux DISPATCHED (a `gtmux spawn`
   ledger entry). `%16` was driven with `gtmux send` into the user's own session, so no
   ledger entry, so no `done`.
2. **Structure.** The wake model is a *selective push*: an event is classified first, and
   only a classification that lands in some wake class (`done` / `asks` / `waiting` /
   `crash` / …) knocks. That `Stop` was notable but belonged to no class — not a tracked
   dispatch, and it asked nothing — so it was silently dropped.
3. **Root cause.** The judgment *"is this event worth waking HQ for?"* was placed in gtmux,
   and **gtmux does not have the context that judgment requires.** Only HQ knows it is
   waiting on `%16`'s install. Lacking that, gtmux can only enumerate rules and guess; each
   scenario the enumeration failed to anticipate vanishes without a trace, and the fix is
   always "add one more entry to the whitelist."

That last sentence is the history of this subsystem: #474-479, #485, bug#1, the 2026-07-18
incident, and #647 are all the same patch applied to different holes. #647 is the sharpest
evidence — the distill/self-check triggers fired correctly for **13 days** into a spool with
no reachable reader, and nobody could tell, because a signal that does not arrive is
indistinguishable from a signal that was never worth sending.

A whitelist cannot be completed by adding to it. The question has to change.

## What changes

From *"does this event deserve a wake?"* to **"is there anything HQ has not consumed?"** —
a question gtmux can answer with no context at all, from two integers.

- **① The watermark (Phase 1, core).** gtmux tracks HQ's CONSUMPTION WATERMARK: how far it
  has read the event stream. When the stream end sits past it for the aggregation window,
  gtmux knocks with a new `unread` class carrying **a count and the pull cursor — no
  importance claim of any kind**. Importance is handed back to the only party that can
  judge it. The debt clears only when HQ consumes; until then the knock repeats. This
  generalizes the mechanism #647 built for two maintenance triggers to the entire stream,
  reusing its shape and its `TestSensorsNoOpWithoutASupervisor` rule (no supervisor → the
  watermark must NOT advance, or an HQ arriving seconds later is declared caught up on
  events it never saw).
  - Noise is bounded by aggregation, not by dropping: `unreadDebounceSec` (**120 s**)
    coalesces a burst into one line, `unreadRepeatSec` (**300 s**) paces the re-knock, and
    the class queues at `PriorityStanding` — behind every blocked agent, first evicted at
    the cap. What is NOT bounded is the guarantee: an unadvanced watermark is a standing
    debt.
  - Consumption is HQ's own act, and only two things count: an **unfiltered**
    `gtmux events --since-seq <n>` read from the HQ home — which is what HQ already does on
    every wake, so the everyday path needs no new discipline — or an explicit
    `gtmux events --ack <seq>`. A `--severity`-filtered read does not (it saw a subset:
    the playbook's *"a filter is a triage shortcut, never your model of the world"* turned
    from advice into mechanism), nor does a read starting AHEAD of the watermark (it
    skipped the range between).
  - HQ's own records are excluded from the count. Every knock lands back in the stream as
    the HQ pane's `UserPromptSubmit`, and its reply as a `Stop`; counting those would make
    the sensor its own event source — knock → two events → debt → knock — forever, on a
    fleet where nothing happened.
- **② Observability (Phase 2).** `gtmux doctor` gains an `event consumption` row beside
  #647's HQ-maintenance rows: how far behind HQ is and for how long, flagged past 20 events
  or 30 minutes. Perception is silent in BOTH directions — a knock that lands leaves no
  trace on any screen the user reads, and neither does one that does not — so without this
  row the next miss is again found only by the commander noticing his question went
  unanswered.
- **③ Classes become priority labels (Phase 3).** `done` / `asks` / `waiting` / … keep
  their job: they say what to look at FIRST and they arrive in seconds. They stop being the
  answer to *what HQ knows about* — that is the watermark's job now. Playbook v14 teaches
  the distinction, teaches that an unconsumed event re-knocks until read, and drops the
  per-class "remember to check for this one too" patches the old model needed (the
  maintenance bullet's "a missed knock is not lost" is now a property of the stream, not a
  fact about two classes).

## Impact

- Specs: `hq-wake-protocol` (the class list is reframed; three requirements added),
  `supervisor-agent` (playbook + doctor).
- Code: `internal/hqwake` (watermark + class + config), `internal/hq` (the sensor, the
  events-command writeback, the doctor read side), `internal/events` (`CurrentSeq` — an
  O(1) stream end, because a per-tick full-log scan would cost more than the events it
  watches), `internal/app/doctor.go`.
- Playbook v13 → **v14**, so existing HQ homes learn the model on the next `gtmux hq`.
- Cost: one wake line per standing backlog per 5 minutes, none when HQ keeps up; one small
  file read per 20 s tick in the caught-up case, one log scan per 5 min while behind.
