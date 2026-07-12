# Tasks — HQ presentation

## 1. Menu-bar popover

- [x] 1.1 AgentStore: decode `role`; expose `supervisor` (first role row) +
      exclude supervisor rows from `sections`/counts-per-section (keep in total).
- [x] 1.2 MenuView: the HQ card between summary and sections — brand pane-grid
      avatar + status badge + task line + relative time; click → focus. Ghost
      "not running — start" state shells `gtmux hq` (detached).
- [x] 1.3 Swift tests: sections exclude role rows; card state decode.

## 2. Mobile radar

- [x] 2.1 types/theme: decode `role`; `sections()` excludes supervisor rows.
- [x] 2.2 RadarScreen: HQ card below the server chip (brand mark + status + task);
      tap → Detail in chat mode. Absent when no supervisor.
- [x] 2.3 jest: sections exclusion + card visibility decode.

## 3. Design docs + spec hygiene

- [x] 3.1 DESIGN.md §3: add the HQ card to the popover structure; MOBILE.md §3:
      add it to the radar layout (per the design-sync rule).
- [ ] 3.2 On merge: sync-specs + archive this change.

## 4. Gate

- [ ] 4.1 make check + swift build -c release + npm run check green.
- [ ] 4.2 Dogfood: live HQ shows as the card on menu-bar; mobile card opens chat.

## 5. Deferred (P2 — NOT built here)

- [ ] 5.1 Card expands into a briefing view fed by GET /api/digest (needs-you
      one-liners) — the full 中控面板.
