## Tasks
- [x] Implement bounded shared filters, cursor context and device summaries with regressions.
- [x] Add native/mobile exploration controls and race/identity/paging tests.
- [x] Update contracts, bilingual docs/specs and archive.

## Acceptance
No live audit records are modified. Counts describe recorded activity, not complete request volume. Go access-specific regressions and 94 mobile tests pass; five deliberately broken guards fail their regression tests and sources are restored byte for byte. Native production components compile and render system filter controls and same-name device summaries at 520/720 points in an isolated synthetic fixture. This is not installed-app interaction or VoiceOver acceptance. Hosted platform gates are reported in the PR. Physical iPhone/iPad layout, keyboard and VoiceOver behavior are not verified in this change.
