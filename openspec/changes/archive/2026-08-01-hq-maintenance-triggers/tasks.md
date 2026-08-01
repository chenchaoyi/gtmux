# Tasks — hq-maintenance-triggers

One PR. The sensors already exist and already fire correctly; this re-points their output
at a consumer, makes the firing auditable, and surfaces staleness.

## A — audit: the trigger lands in the event stream

- [x] A1. `internal/events`: add `ControlPrefix` + `IsControl(Record)` — a gtmux-authored
      control record (`gtmux:*`), distinguishable from fleet activity.
- [x] A2. `internal/events`: `Format` renders a control record as
      `HH:MM:SS  [CONTROL <event>]  <summary>` instead of an empty lifecycle row, so a
      `gtmux events` reader can actually see it.
- [x] A3. `internal/hq`: one `raiseMaintenance` helper — append the control record to the
      JOURNAL (the feed daemon spools it on its normal tail, so no hand-written second
      copy) and deliver the wake line. Both sensors go through it.

## B — delivery: the trigger knocks

- [x] B1. `internal/hqwake`: `ClassDistill` / `ClassSelfCheck`, both at `PriorityStanding`
      in the priority table.
- [x] B2. `internal/hq/distill.go` + `selfcheck.go`: raise through `raiseMaintenance`,
      dropping the direct `hqfeed.EmitControl` (`feed-degraded` / `wake-degraded` keep
      theirs — the journal path is exactly what may be broken when they fire).
- [x] B3. Self-feed guard: exclude control records from `distillSensor`'s accrued count and
      from `recentAttentionEvent`, so a raised trigger can neither satisfy its own
      zero-change gate nor suppress its own idle condition.

## C — cadence: the spool floor (layer ③(c), previously deferred)

- [x] C1. `shouldDistill` takes the pending-candidate count; fires on `pending >= N`
      (default 5) behind the rate limit, and a non-empty spool satisfies the zero-change
      gate. Weekly / volume floors and the ≤1/day rate limit unchanged.

## D — observability

- [x] D1. `internal/hq`: export `MaintenanceStatus()` — last-run times + a pure
      `maintenanceState(now, lastAt, floor, grace)` verdict (never-run / ok / slipped).
- [x] D2. `internal/app/doctor.go`: an "HQ maintenance" section, present only when an HQ
      home exists.
- [x] D3. `gtmux capture --list`: a one-line header with the last-distill age.

## E — charter + docs

- [x] E1. `hqPlaybookVersion` 11 → 12 + a `v12 —` version-history entry.
- [x] E2. `hqInstructions`: teach `distill` / `self-check` in the wake-class list; both
      rituals now say the trigger arrives as a wake AND sits in `gtmux events`, so a
      missed knock is recoverable by pull.
- [x] E3. `docs/cli.md`: the two classes in the wake-class table (enforced by
      `check-design.sh` §7) + how to audit "did it run?" from `gtmux events`.

## F — tests + gate

- [x] F1. `shouldDistill` table: weekly / volume / spool fire; rate-limited and
      zero-change do NOT. Both directions covered.
- [x] F2. `shouldSelfCheck` / `recentAttentionEvent`: a control record does not count as
      attention (the regression B3 prevents).
- [x] F3. `events.IsControl` + `Format` control rendering.
- [x] F4. `PriorityOf` for the two new classes = `PriorityStanding`.
- [x] F5. `maintenanceState` verdicts (never-run / ok / slipped boundaries).
- [x] F6. `make check` green (gofmt · vet · staticcheck · `go test -race`) +
      `scripts/check-design.sh`.

## G — spec lifecycle

- [x] G1. Sync the deltas into `openspec/specs/{supervisor-agent,hq-wake-protocol}`.
- [x] G2. Archive this change (same PR — `changes/` holds only in-flight work).
