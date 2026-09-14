# Design record — menubar-hq-report

Three directions were drawn (canvas "菜单栏 HQ 模块", 2026-09-14) and the commander chose A.

- **A — the card expands in place** (chosen). The phone's report table inside the existing
  bordered panel; head click still jumps; ⌄ discloses; auto-open on red. Cost: one more
  control on the head. Gain: every ask answered without a new window or a permanent row.
- **B — three labelled doors in a row under the card.** Smallest change, live numbers on the
  doors. Rejected: a permanent 30pt row of the popover's most expensive pixels, which
  DESIGN §12 already refused once for two full-width buttons; no room for owed / did.
- **C — a standalone HQ window mirroring the phone page.** Richest reading surface.
  Deferred: the jump to the pane gains a click; the supervisor's quote and act stream have
  no CLI read yet. A's table is C's left column if it is ever built.

## Decisions

- **D1 · The table is the phone's table.** Keys, order and absence rules are ported from
  `mobileapp/src/screens/hqHeaderModel.ts`, not redesigned; the Swift resolver is pure and
  tested in both languages so the two surfaces cannot drift in silence.
- **D2 · Machine leads only when red.** At red the row is first, red, and may wrap to two
  lines (a truncated number is worse than a wrapped one — the phone's rule for its context
  row). At amber it sits in its ordinary place in amber. Healthy: plain grey, still a door.
- **D3 · Knowledge carries the debt.** The phone splits "owed" and the "knowledge" door;
  the popover has 420pt and folds them: `478 entries · 7 waiting on you · oldest 16d`.
  Amber only past `PROMOTION_STALE_SECS` (14 days), the line doctor uses.
- **D4 · The expansion polls only while open.** The card's data (promotions, entry count,
  board timestamp, 24h acts) is fetched on expand and every 60s while expanded; collapsed
  costs nothing beyond the existing 25s resource poll, which now keeps the whole machine
  snapshot rather than the tier alone.
- **D5 · Acts are read with `--acts`, never from the HQ home.** The app's events read runs
  from its own cwd, so it never advances HQ's consumption watermark; the filter lives in
  the core (`events.IsSupervisorAct`) so the CLI and the API agree on what an act is.
- **D6 · The machine tab reads, it does not reclaim.** Orphans are listed with the core's
  hint text. A reclaim action would be the first thing in the reader that changes the
  machine; if wanted, it is its own change with its own confirmation and audit record.
