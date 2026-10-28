## Implementation
- [x] Preserve the original worktree and integrate existing browser changes with current-main provider/input behavior.
- [x] Audit DSH and current browser trace flows with inspected screenshots.
- [x] Implement stable public event indexing and recorded tool detail rendering.
- [x] Fix external conversation entry labels and read-only trace navigation.
- [x] Improve compact/mobile trace navigation, overview controls and update stability.
- [x] Test provider/session/state/appearance/viewport combinations and fix regressions.
- [x] Document native App design and feasibility with explicit delivery boundaries.
- [x] Sync bilingual design/specs, validate and archive.

## Acceptance
All input mutation tests use synthetic panes and fixtures. No real agent receives a test message. Record simulated, live-reference and physical-device limits separately.

## Verification
- Browser regression: 50 cases (27 existing/workspace cases, 19 trace cases, 4 profile cases), including exact-pane identity with multiple HQ sessions.
- Public model and page logic: 96/96 passing.
- Transcript/server targeted race tests, embedded Web route checks and cgo-free build passed.
- Native menubar workspace/pairing/guest compile and selected tests: 6/6 passing.
- Live DSH was inspected read-only; final evidence uses synthetic panes.
- Safari simulator load was observed, but its input tool did not complete interaction. Physical devices, VoiceOver and native App Trace are not accepted by this change.
- Native App Trace remains a bilingual design proposal. No production service or App was replaced.
