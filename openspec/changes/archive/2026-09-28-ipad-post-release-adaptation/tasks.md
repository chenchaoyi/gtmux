## Implementation

- [x] Audit user-visible mobile changes since the live 1.0.30 release against the iPad shell.
- [x] Wire HQ's task row, sheet and pane navigation through `HQView`.
- [x] Align demo task panes with the sample fleet and add an iPad navigation flow.
- [x] Keep demo task status in sync with the sample permission arc.
- [x] Cap new task and share sheets on regular canvases; use palette text colour.
- [x] Keep the split shell from initially opening HQ as an ordinary terminal detail.
- [x] Sync the canonical mobile spec and bilingual mobile design docs.
- [x] Run mobile type, lint, full Jest, focused tests, iPad landscape simulator flow and design gate.

Portrait rotation was rejected by this simulator's XCUITest driver; no physical iPad
is connected. These remain device acceptance checks, not claimed as passed.
