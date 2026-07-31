# Widen the menu-bar popover to a companion-app baseline (320 → 420)

## Why

The menu-bar popover is too narrow. At the current **320pt** (`Theme.Size.popoverWidth`,
DESIGN §3), the single-line rows — session/task, and especially the HQ card's goal/
last/ask digest line — tail-truncate early, so a lot of the useful text disappears
behind `…` even when there is screen room to show it.

A companion menu-bar app solved the same problem: it sets a popover `.frame(width: 420)`,
with 360/430 tried and **420** the calibrated value. We adopt that same width so it reads
as one visual family and gtmux's digest text has room to breathe.

## What Changes

- **Popover width `320 → 420`.** Bump the single design token
  `Theme.Size.popoverWidth`; every row inherits it (rows are `.frame(width:
  popoverWidth)` or `maxWidth: .infinity` inside it). No per-row width is hardcoded.
- **Long content (goal / last / ask).** No layout change needed: all of these rows
  are already `.lineLimit(1).truncationMode(.tail)` (single line, tail ellipsis), and
  the one wrapping case — `WaitingReplyView` reply options at `.lineLimit(2)` — only
  benefits. Wider = strictly more text before truncation, no reflow risk.
- **Adaptive width — evaluated, declined.** Since every row is single-line
  tail-truncated, no content "wants" to be wider than the frame; a content-driven /
  max-width popover would add jitter for zero legibility gain, and the companion app
  itself uses a fixed 420. We keep a **fixed constant** (the design-token model).
- **Spec + docs synced.** DESIGN §3 size table (`popover 宽`) and this capability spec
  record 420; a test pins `popoverWidth == 420`.

Out of scope: the standalone Automation-permission / first-run card (`FirstRunView`,
its own 360pt window, not the popover) is left unchanged.

## Impact

- Affected code: `macapp/Sources/GtmuxBar/Theme.swift` (one constant).
- Affected specs: `menu-bar-app`.
- Affected docs: `docs/design/DESIGN.md` §3.
- Affected tests: `macapp/Tests/GtmuxBarTests/ModelTests.swift`.
- No CLI / `agents --json` contract change; menu-bar-only, purely visual.
