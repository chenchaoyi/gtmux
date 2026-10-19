## Implementation

- [x] Preserve descriptive Codex tool metadata in bounded summaries.
- [x] Add structured activity presentation to phone/iPad and Web.
- [x] Verify ordering, disclosure, escaping, summary bounds and layout; update paired design docs/specs.

## Acceptance

- Author checks: full Go make check; mobile TypeScript/lint/Jest (170 suites, 1761 passed, 15 existing skips); Web 62 unit tests and 9 rendered Chromium flows, including 320px/1280px, long unbroken inputs, plain-text escaping and update/disclosure behavior. Existing suite-level skip remains.
- Parser regression was red before the fix; reversing native row order or interpreting Web detail as HTML made the new tests fail; mutations restored byte-for-byte.
- iPhone simulator (light device chrome, dark Chat canvas): real app build, folded/open card and selectable descriptions. iPad simulator (dark, largest accessibility text category): same build and open card. Synthetic API only, no actual command execution. Browser manual inspection: 320px, no horizontal overflow or app console errors.
- Not covered: physical devices, spoken VoiceOver, Android runtime, real agent execution/output, or independent review. This contract has no per-call result; no success, duration, exit code or stdout/stderr is inferred.
