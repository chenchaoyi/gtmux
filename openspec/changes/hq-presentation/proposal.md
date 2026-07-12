# HQ presentation — the supervisor gets its own layer, not a list row

## Why

The supervisor (`role:"supervisor"`, shipped in the supervisor-mvp change) is a
META-level session — it watches the fleet. Stacking it parallel with the
sessions it watches (in the waiting/working/idle sections) demotes it and
pollutes the section semantics: a supervisor is near-always "working", so it
permanently squats the working section above real work. Per DESIGN §3 ("层级即
一切"), it deserves its own layer. (User direction 2026-07-12: "它不应该跟其他
session 平行地堆在一起".)

## What Changes

- **Menu-bar popover**: a persistent compact **HQ card** between the header
  summary and the section list. Running → gtmux BRAND pane-grid mark as the
  avatar (§12/§15 — the supervisor is gtmux's own concept, visually distinct
  from agent avatars) + the standard status badge + its task line (from the
  existing `agents --json` row; zero new data), click = `gtmux focus`. Not
  running → a quiet ghost slot "中控 · 未运行 — 点击启动" that shells
  `gtmux hq` (the app stays a pure CLI consumer). `role:"supervisor"` rows are
  EXCLUDED from the grouped sections (and from the summary counts' sections,
  staying in the total).
- **Mobile radar**: the same HQ card below the server chip; tap opens the HQ
  Detail in CHAT mode (conversing with the supervisor from the phone is the
  killer path). Excluded from the SectionList the same way.
- **P2 evolution (spec'd as deferred, NOT built here)**: the card expands into a
  briefing view fed by `GET /api/digest` (needs-you one-liners) — the full
  "中控面板". Step 1 deliberately renders from the agents row only.

## Capabilities

### Modified Capabilities
- `menu-bar-app`: the HQ card + section exclusion.
- `mobile-app`: the HQ card + section exclusion + tap-to-chat.

## Impact

- macapp: MenuView (card + exclusion), AgentStore (role decode — field already
  additive), AppDelegate (launch `gtmux hq` action).
- mobileapp: RadarScreen (card), theme.ts sections() (exclusion), AgentRow reuse.
- Design docs: DESIGN.md §3 structure + MOBILE.md radar section get the HQ card
  (per the "改了设计就同步更新规范" rule).
- No server/CLI change; the `role` field already ships.
