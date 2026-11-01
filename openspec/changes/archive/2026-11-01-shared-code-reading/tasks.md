# Implementation
- [x] Share native code presentation across Chat, Trace, Markdown and expanded reader.
- [x] Add bounded static wrapper extraction and syntax-preserving Python compact formatting.
- [x] Preserve call-group ownership in the shared model and native/browser rendering.
- [x] Synchronize capability specs and bilingual design guidance.
- [x] Complete focused, full mobile, browser and design/spec checks.
- [x] Record rendered acceptance and explicit native interaction limits.

## Acceptance
App full checks pass (2126 tests,15 configured skips), focused component/formatter actions and Python ast.parse equivalence pass. Browser full run109/113 with four outdated horizontal/copy assumptions corrected; all ten affected/new flows pass locally, final-head CI must independently validate the entire suite. Current-JS rendering in an isolated152/RN0.86 simulator host covers code cards and call hierarchy. AX exposes only one root/zero targets, so native touch, VoiceOver and real-device interaction remain unverified. No release or installation of the product is part of this change.
