# HQ command center — the supervisor's detail is a 司令部, not a normal session

## Why

Tapping the gtmux HQ card opens the GENERIC agent Detail (Chat/Terminal
segmented) — the same view as any worker. But HQ is not a worker; it is the
machine's command post. The user's direction (2026-07-12): the HQ detail should
(1) integrate the primary 中控 command input, (2) let the user sense ALL tmux
windows' situation from this one window, and (3) send instructions well — "真正
成为机器上的司令部,所有命令均来自于 HQ". This is the deferred "HQ card → digest
briefing view" (supervisor-mvp P2) upgraded into a full command console.

## What Changes

- **Mobile: a dedicated HQ Command Center screen** replaces the generic Detail
  when the opened agent is `role:"supervisor"`. Three stacked zones:
  1. **Status strip** — "gtmux HQ" + connection + a live fleet+plan line
     (`3 needs-you · 2 working · 6 idle · week 60%`), from `/api/digest` counts
     + `/api/usage` limits.
  2. **Fleet board** — the situational-awareness list from `/api/digest`,
     needs-you→working→idle, each row: state badge · loc · agent · goal (one
     line) · ask (if waiting). Tapping a row SELECTS it (its loc is bound to the
     command bar's context + per-target quick actions appear); long-press JUMPS
     to that pane's own Detail. This is the "感知所有 tmux 窗口" surface.
  3. **Command console** — the conversation WITH gtmux HQ (reusing ChatView) +
     a prominent command bar: free text → HQ, plus quick-command chips
     (`现状` · `谁在等我` · `用量/额度`; when a fleet row is selected:
     `让它继续` · `看它在干嘛` · `帮我回复它`). ALL commands route through HQ
     (the supervisor decides + drives); a direct pane jump stays as the
     long-press escape hatch.
- **Mobile client**: add `digest()` (+ reuse for usage/limits) over the existing
  authed API. No new server endpoints — `/api/digest` + `/api/usage` already ship.
- **P2 (deferred)**: a menu-bar HQ command popover (lighter — the phone is the
  primary command surface); voice command input; HQ-authored one-tap action
  buttons parsed from its replies.

## Capabilities

### Modified Capabilities
- `mobile-app`: the supervisor opens the HQ Command Center (fleet board +
  command console), not the generic Detail.

## Impact

- mobileapp: a new `HQScreen`/`HQCommandCenter` (RadarScreen routes the
  supervisor there instead of `Detail`); reuses ChatView + the composer; a
  DigestBoard component over a new `client.digest()`.
- No server/CLI change (the digest + usage contracts already exist).
- The command model is HQ-mediated (you command HQ; HQ commands the fleet),
  matching "所有命令均来自于 HQ"; direct control stays available via long-press.
