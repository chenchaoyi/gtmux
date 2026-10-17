## Tasks
- [x] Implement compact localized native/mobile rows and presentation tests.
- [x] Fix verified access classification/attribution gaps and add inventory/behavior/privacy regressions.
- [x] Update API, paired English/Chinese docs and capability deltas; archive the implemented change.

## Acceptance
Targeted Go audit tests, mobile TypeScript/lint and 10 mobile tests passed. Five deliberate regressions (export sampling, desktop target coalescing, missing creation pane, unclassified registered route, ignored UI language) were rejected and restored byte-for-byte. The production native log components were compiled and rendered with synthetic data at 520/720pt in Chinese light and English dark modes. Required full checks and hosted exact-head CI are recorded in the PR.

No live server audit data is changed. Full native/iOS compilation runs in CI where local disk is constrained; this is not installed-app, physical-device or VoiceOver acceptance.
