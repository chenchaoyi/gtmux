# hq-self-rotate — HQ judges its own session's fitness, and rotates itself

## Why

On 2026-08-03, in an HQ session that had been running a long time at high context, **HQ
read its own output as the commander's input.** It found a line saying, in effect, *"that
message was from me, don't worry"* — and withdrew a suspicion it had correctly raised. The
line was not the commander's. It was HQ's own previous turn.

The event stream settles it, because the two carry different event TYPES:

| seq | event | pane | what it actually was |
| --- | --- | --- | --- |
| 7196 | `UserPromptSubmit` | `%4` | the commander's real input |
| 7210 | `Stop` | `%4` | HQ's own turn output — the "it was me" line lives here |

So this is not a reasoning slip that more care would have prevented. **In a long, near-full
session the boundary between what HQ produced and what reached it from outside degrades**,
and the degradation is invisible from the inside: the failing faculty is the same one that
would have to notice the failure. The commander noticed and told HQ to rotate — which is
precisely the judgment that must not be his to make.

There is a second, structural reason gtmux has to be the one to raise it. **HQ has no clock
of its own.** Between wakes it is not running. "HQ should rotate when it gets heavy" is,
without a resident trigger, an instruction delivered to nobody — the exact failure #647
documented, where correctly-fired distill/self-check sensors ran zero passes for 13 days
because nothing resident carried them to a reader.

## What changes

Same family as the consumption watermark (#650/#651): **put the judgment where the context
is, and guarantee the signal reaches it.** The difference is the object under observation —
there it was the event stream, here it is HQ itself.

- **① A session-health sensor (`internal/hq/selfrotate.go`).** The serve slow-tick reads
  three facts about the HQ pane's own session and compares them to conservative thresholds:
  **ctx occupancy** (the live context fraction the digest already computes), **session age**
  (from the transcript's FIRST message, so it survives a serve restart rather than resetting
  to zero), and **turn count** (HQ-pane `UserPromptSubmit` records since the session began).
  Any one over its threshold is a breach.
- **② A `self-rotate` wake class.** On a standing breach the sensor knocks
  `» gtmux·self-rotate  ctx 82% · 14h · 380 turns │ …`, at `PriorityStanding` — behind every
  blocked agent, evicted first at the cap, because it re-fires on its own cadence. The debt
  clears the way the watermark's does: **only by the act itself.** Nothing gtmux does
  advances it; a NEW session id does.
- **③ HQ performs the rotation, unattended.** Playbook v16 teaches the three steps in order
  — ① bring the situation board and knowledge base current (they ARE the successor's
  briefing), ② hand off, ③ rotate — and the commander is not in the loop for any of them.
  `gtmux hq --rotate` is the mechanism: gtmux resolves the HQ pane itself and delivers the
  agent's own reset input, so HQ never touches tmux directly and the HARD role whitelist
  (HQ runs no concrete command) stands.
- **④ Observability (`gtmux doctor`).** An **HQ session health** row joins the HQ
  maintenance group beside `event consumption` / `knowledge distill` / `HQ self-check`,
  showing `ctx 41% · 3h · 88 turns` and flagging a standing breach. Without it the failure
  is silent in both directions, exactly as the consumption lag was.

### Why this is its own sensor, not a `self-check` item

`self-check` and `self-rotate` look adjacent and are not. **`self-check` audits HQ's
PRODUCTS** — ledger, memory, feed, log health; **`self-rotate` audits the JUDGE** — whether
this session is still fit to make any of those calls. Four consequences make merging them
actively worse:

1. **Opposite prescribed volume.** `self-check`'s charter is *"clean silently, brief only on
   real action"*. Rotation is a deliberate, disruptive handoff. Folding them means either
   ritual hygiene starts interrupting, or rotation inherits "stay silent" — and silence is
   the failure mode being fixed.
2. **Different clocks, and sharing one loses signal.** `self-check` is time-driven and
   rate-limited to ≤1/h. `self-rotate` is threshold-driven on monotone signals, cleared by
   exactly one event. On a shared limiter a self-check that just ran would suppress a
   rotation knock for an hour.
3. **Doctor cannot render a merged verdict.** The row has to say `ctx 82% · 14h` for the
   user to trust or dispute it. A merged sensor has one staleness timestamp and no such
   value.
4. **Blast radius.** Turning off a knock that misjudges rotation must not also turn off
   artifact hygiene.

They stay siblings: both maintenance-family, both standing priority, both raised by the
resident tick, both auditable from the stream.

## Impact

- Specs: `hq-wake-protocol` (class list + the new class's requirement), `supervisor-agent`
  (the playbook ritual + the doctor row).
- Code: `internal/hqwake` (class, priority, four config knobs), `internal/hq`
  (`selfrotate.go` sensor + `--rotate` + slow-tick wiring + playbook v16),
  `internal/transcript` (`FirstMessageTime` — the head-read twin of `LastMessageTime`),
  `internal/app/doctor.go`.
- Playbook v15 → **v16**, so existing HQ homes learn the ritual on the next `gtmux hq`.
- Defaults are deliberately conservative — a knock nobody believes is worse than none:
  **ctx ≥ 75%**, **age ≥ 12 h**, **turns ≥ 300**, re-knock every **1800 s**. Each is
  independently disableable (`≤ 0`) via `hqWake.selfRotate*`.
- Cost: nothing while healthy (one small marker read per tick); one log-head read, one usage
  snapshot and one event-delta scan per evaluation interval (default 300 s) while an HQ is
  live.
