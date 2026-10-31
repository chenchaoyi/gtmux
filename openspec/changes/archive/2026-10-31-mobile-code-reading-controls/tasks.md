## Implementation
- [x] Keep Diff vertical indicators visible at every horizontal position without losing virtualization.
- [x] Replace visible help paragraphs with functional scroll and information icons.
- [x] Cover drag, native-scroll synchronization, accessibility adjustments and exact-copy/partial-content regressions.
- [x] Check native layouts using the warm isolated fixture and record interaction evidence limits.
- [x] Sync design, user docs and canonical spec; archive.

## Acceptance
The current components render in the existing isolated Release native host; no new native dependency build was required. Programmatic native scroll and actual React callbacks configure screenshots, including a fixed visible Diff indicator at horizontal left/middle/right. Drag math, offset synchronization, bounded/virtualized rendering, partial/summary explanations and copy are covered by component tests. Simulator AX remains unavailable: native finger dragging, native tap navigation, VoiceOver and physical-device acceptance are unverified. No release/install on the user device is included.
