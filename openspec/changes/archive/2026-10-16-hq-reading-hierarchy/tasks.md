## Implementation
- [x] Introduce scoped HQ typography and apply it to workspace navigation, reading, resources and input.
- [x] Classify known activity kinds with semantic symbols, labels and an inclusive category filter.
- [x] Replace repeated phone settings icons with existing symbols matching their function.
- [x] Verify taxonomy, symbols, filtering, scoped fonts, layout bounds and relevant regressions; capture isolated render evidence where possible.
- [x] Update English/Chinese design documentation and sync/archive the spec.

## Acceptance
Live native tool startup currently fails with `Sky Computer Use native pipe startup failed`. Offscreen rendered evidence does not establish real VoiceOver behavior. Local disk is constrained; avoid iOS/native release builds.

Verification: 28 HQ tests, five composer-key tests, actual SF Symbol lookup and light/dark fixture rendering passed. Offscreen captures omit AppKit sidebar contents; sidebar visual acceptance and actual VoiceOver remain unverified. Mobile native/device visuals remain unverified. Full gates and mutation checks are recorded in the PR.
