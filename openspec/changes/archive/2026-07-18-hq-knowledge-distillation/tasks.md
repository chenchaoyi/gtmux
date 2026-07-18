# Tasks: hq-knowledge-distillation

## 1. Control record plumbing (mirror self-check — a feed control record, NOT a wake class)

- [x] 1.1 Add `ControlDistill = "gtmux:distill"` in `internal/hqfeed/hqfeed.go`
  (beside `ControlSelfCheck`).
- [x] 1.2 Render `ControlDistill` in `internal/app/hqfeedcmd.go`'s control switch
  (add to the `ControlReconcile/FeedDegraded/SelfCheck` case). No `hqwake.Class` —
  distill is a low-urgency maintenance control record like self-check, not a typed wake.

## 2. The distill sensor (LLM-free, slow-tick)

- [x] 2.1 New `internal/app/distill.go`, modeled on `selfcheck.go`:
  - Timing consts: `distillMinInterval` (rate limit), `distillWeeklyFloor`,
    `distillVolumeFloor` (new-event count).
  - `last-distill` marker helpers (`state.Dir()/hq-feed/last-distill`, storing the
    watermark seq + time).
  - `shouldDistill(now, lastAt, lastSeq, curSeq, notableSince int) (bool, string)` —
    PURE decision: rate gate → volume floor (curSeq-lastSeq ≥ floor) → weekly floor →
    else skip; zero-change gate (notableSince == 0 ⇒ never fire).
  - `distillSensor(now)`: gate on a live HQ pane, cheap rate gate first, then read the
    current event seq + notable-since count, decide, advance the watermark, `EmitControl`.
- [x] 2.2 Call `distillSensor(now)` from `slowTickEval()` next to `selfCheckSensor`.
- [x] 2.3 Pick `distillVolumeFloor` conservatively for the current 20 MB (~×2) event
  log; add a comment noting it must be reconciled with the events-retention work.

## 3. Seed the playbook (PROMPT)

- [x] 3.1 In `internal/app/hq.go` `hqInstructions`: add a `distill` control-record
  bullet next to the self-check bullet (line ~707), one clause — parallel to how
  self-check is taught (a `[CONTROL gtmux:distill]` record, not a wake class).
- [x] 3.2 Extend the Knowledge-base § to formalize the distillation ritual: on a
  `distill` control record, distil the event delta since the last distill into the KB
  (update-over-append), prune stale + merge dupes; default SILENT, one line on real
  curation; charter-level → flag a seed/spec update; never store secrets. Keep it tight.
- [x] 3.3 Bump `hqPlaybookVersion` 6 → 7 (mandatory: `hqInstructions` changed).

## 4. Tests

- [x] 4.1 `internal/app/distill_test.go`: `TestShouldDistill` table — rate-limited;
  volume floor fires; weekly floor fires; zero-change gate skips; precedence
  (volume before weekly).
- [x] 4.2 `internal/app/hq_test.go`: assert the seeded playbook contains the `distill`
  control-record bullet + the distillation ritual clause, and `hqPlaybookVersion == 7`
  (mirror the existing playbook-content assertions).

## 5. Docs / spec / gate

- [x] 5.1 `make check` green (gofmt + vet + staticcheck + `go test -race`) — includes
  check-design.sh. (distill is a control record like self-check, not a wake class, so
  no docs/cli.md wake-table entry / class-parity obligation.)
- [x] 5.2 `openspec validate hq-knowledge-distillation --strict` passes.
- [x] 5.3 Sync deltas into `openspec/specs/supervisor-agent/spec.md` and archive the
  change (same PR / next), keeping checkboxes truthful.
