## Implementation
- [x] Audit the four result classes and distinguish completion from findings.
- [x] Add CLI final findings summary and reuse it after repair passes.
- [x] Add Mac summary, classification help and original-row navigation.
- [x] Verify filtering, order, duplicate-label provenance, retry persistence, component rendering and native window sizing.
- [x] Update paired docs and complete required checks.

## Acceptance
No actual repair, rotation, installation or release. Summary components were visually inspected in both languages; a native hosting
window verifies the 560×600 content bounds. Model tests verify exact source-row
identities and focused detail expansion. ImageRenderer cannot reliably render this
full macOS ScrollView/native link view. Native UI automation timed out twice for an
isolated synthetic window; end-to-end clicking, installed-app layout and VoiceOver
acceptance remain unverified. No unavailable screenshot is counted as acceptance.
