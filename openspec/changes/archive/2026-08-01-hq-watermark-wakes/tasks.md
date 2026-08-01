# Tasks — hq-watermark-wakes

One PR, three phases. The mechanism is not new: #647 built exactly this shape for two
maintenance triggers (watermark marker, pure decide, no-op without a supervisor). This
generalizes it from two triggers to the whole stream and reuses its structure and naming.

## Phase 1 — the watermark and the unconsumed knock (core)

- [x] 1.1 `internal/events`: `CurrentSeq()` — the stream end from the counter file, O(1).
      `LatestSeq` scans every retained record, which is fine once per pull and far too
      expensive on a 20 s tick.
- [x] 1.2 `internal/hqwake/watermark.go`: `Consumed` / `Consume` / `ConsumeRead`, with the
      three invariants — UNSET is distinct from 0 (so a fresh install adopts the stream end
      instead of being told it is thousands behind), advancement is MONOTONIC, and a read
      counts only when CONTIGUOUS with what HQ already consumed.
- [x] 1.3 `internal/hqwake`: `ClassUnread` at `PriorityStanding`; `unreadDebounceSec` (120)
      and `unreadRepeatSec` (300) config knobs with defaults.
- [x] 1.4 `internal/hq/unread.go`: the pure `unreadDecide` (aggregate → knock → repeat →
      restart on a moved watermark) plus the sensor, gated on a live HQ pane exactly like
      the maintenance sensors, and excluding the HQ pane's own records from the count (the
      anti-self-feed rule).
- [x] 1.5 `internal/hq/slowtick.go`: run it last of the wake-raising sensors, so it only
      ever speaks about what no other class claimed.
- [x] 1.6 `internal/hq/eventscmd.go`: the writeback — an unfiltered `--since-seq` read from
      the HQ home consumes; a filtered or skip-ahead read does not; `--ack <seq>` is the
      explicit form, HQ-home-only, clamped and monotonic. Usage text in en+zh.

## Phase 2 — observability

- [x] 2.1 `internal/hq`: `ConsumptionStatus` — unread count, standing age, verdict, from
      disk (pure read, safe on any host).
- [x] 2.2 `internal/app/doctor.go`: an `event consumption` row in the HQ section, flagged
      past 20 events or 30 minutes.

## Phase 3 — classes become priority labels

- [x] 3.1 Playbook v14: classes are priority not coverage; the watermark guarantee; what
      does and does not count as consumption; `--ack`; the `unread` handling rule.
- [x] 3.2 Retire the per-class "remember to check for it" patch — the maintenance bullet's
      recoverability is now a property of the stream, stated once.
- [x] 3.3 `docs/cli.md`: the class-table row, a "why nothing goes missing" section, and the
      `--ack` / writeback rules in the `gtmux events` section.

## Tests

- [x] T1 Unconsumed events DO knock (`TestUnreadKnocksThenStopsOnConsumption`), including
      that the knock carries no importance claim and does not advance the watermark itself.
- [x] T2 Consumption stops it — for good, not for one interval (same test), and an
      unconsumed debt re-knocks at the repeat interval
      (`TestUnreadRepeatsWhileUnconsumed`).
- [x] T3 No supervisor → no knock, no watermark movement, no sensor state
      (`TestUnreadSensorNoOpWithoutASupervisor`, mirroring
      `TestSensorsNoOpWithoutASupervisor`).
- [x] T4 The anti-self-feed rule (`TestUnreadIgnoresHQsOwnEcho`) and control records still
      counting as owed work (`TestUnreadCountsControlRecords`).
- [x] T5 Writeback rules: unfiltered consumes, filtered does not, skip-ahead does not,
      `--ack` clamps/monotonic/HQ-only (`internal/hq/eventscmd_test.go`).
- [x] T6 Watermark invariants + standing priority + config defaults
      (`internal/hqwake/watermark_test.go`).
- [x] T7 Doctor row (`TestHQConsumptionCheck`) and the playbook model
      (`TestPlaybookTeachesTheConsumptionWatermark`).

## Close-out

- [x] C1 `make check` green; `scripts/check-design.sh` green (the new class is taught in
      both docs/cli.md and the seeded playbook).
- [x] C2 Spec deltas synced into `openspec/specs/{hq-wake-protocol,supervisor-agent}` and
      this change archived.
