## Implementation
- [x] Implement the shared compact working disclosure and bounded preview.
- [x] Verify refresh/turn/session state, elapsed time, bilingual controls and existing waiting-input behavior.
- [x] Update paired design guidance, synchronize the capability and archive the change.

## Acceptance
Author mobile gate: TypeScript, ESLint and 182 Jest suites (1,862 tests) passed; one suite and 15 tests intentionally skipped. An isolated synthetic iPhone runtime renders the real ChatView over the published v1.0.145 simulator native runtime with a new JS bundle; this is not a fresh native build or device install. Appium/XCTest verified real expand/collapse taps, bilingual actions, refresh retention and new-turn reset on the synthetic phone fixture. Native-only/read-only branches are component-tested. iPad rendering, real devices and spoken VoiceOver were not exercised. Physical-device and spoken VoiceOver acceptance remain unverified. Web has no automatic working preview to change.
