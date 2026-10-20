## Implementation

- [x] Add scoped per-agent profile collection and bounded native readers.
- [x] Capture allowed native-hook facts and sanitized runtime evidence.
- [x] Wire an owner-only profile endpoint for verified conversations.
- [x] Implement participant dialogs and independent, attached reply folding.
- [x] Verify source isolation, historical boundaries, privacy and UI interactions.
- [x] Align API, bilingual docs and capability specs with delivered behavior.

## Acceptance

Author regressions, independent review, simulator visuals, physical devices and
VoiceOver speech are separate evidence and must be reported separately.

Native Release phone/iPad runs used synthetic conversations. Extended Dynamic Type
uses a one-column facts layout; a fresh phone capture and touch run passed after
the reflow fix. Real model turns, independent review, physical devices, and
VoiceOver speech remain unverified. Hook-only current snapshots do not reconstruct
historical details. No user hook configuration was changed by this work.
