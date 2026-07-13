# Tasks — Widen the menu-bar popover (320 → 420)

- [x] 1. Measure MPBar's target width (`multipilot-companion` `MenuView.swift` → `.frame(width: 420)`)
- [x] 2. `Theme.Size.popoverWidth` 320 → 420, with a comment citing the MPBar baseline
- [x] 3. Confirm long content (goal/last/ask) needs no layout change — all rows already single-line tail-truncated
- [x] 4. DESIGN §3 size table: `popover 宽` 320 → 420
- [x] 5. `menu-bar-app` spec: record the width + single-line-truncation legibility requirement
- [x] 6. Test: pin `Theme.Size.popoverWidth == 420` in `ModelTests`
- [x] 7. `cd macapp && swift build -c release && swift test` green
- [x] 8. `openspec validate --specs --strict` green; branch → PR
