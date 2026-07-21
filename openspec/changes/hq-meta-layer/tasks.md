# Tasks

## 1. Notification role-gating (PR-B, Go core)
- [x] 1.1 Add `Role string` to `server.AgentStatus`; thread `Role: p.Role` in `serve.go`.
- [x] 1.2 In `events.go`, exclude `Role == "supervisor"` from the Waiting/Working/Idle
      tally and from setting `WaitingTitle`/`WaitingSession` (the headline is workers).
- [x] 1.3 In `hook.go`, when the firing pane is the supervisor (`hqpane.FindOther` →
      self), suppress the routine `done` notification; keep `input` (needs-your-decision).
- [x] 1.4 Tests: events tally excludes a supervisor row (counts + headline); a helper
      pins the hook's HQ-done suppression decision.

## 2. HQ card redesign (PR-C, menu-bar + mobile)
- [ ] 2.1 Menu-bar `MenuView.swift`: delete `fleetPips`; subtitle = a synthesized
      intelligence headline over the worker agents (naming the waiter + "其余 N 正常" /
      "都正常"), red/amber when it needs you.
- [ ] 2.2 Mobile `HQCard.tsx`: delete the pips block; same synthesized headline; update
      `HQCard.test.tsx`.
- [ ] 2.3 A shared-shape deterministic headline function per surface, unit-tested
      (waiting → names first + rest count; quiet → "all normal").
- [ ] 2.4 Docs: mockup §12 (drop "fleet pips ★ chosen", show the headline), DESIGN.md +
      MOBILE.md HQ sections.

## 3. Verify
- [ ] 3.1 `make check` + mobile `npm run check` + `scripts/check-design.sh` green.
