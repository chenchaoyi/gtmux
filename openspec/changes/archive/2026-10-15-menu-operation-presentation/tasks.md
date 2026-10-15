# Tasks

- [x] Add a narrow presentation coordinator with explicit dismissal and window-lifetime cleanup.
- [x] Wire Restore and automatic release notes without reopening a user-dismissed menu.
- [x] Test lifecycle, focus/dismissal boundaries and stale callbacks; document both languages and sync/archive specs.
- [x] Run core/native/design checks and bounded isolated AppKit acceptance.

## Acceptance

Full core check and 438 native tests passed. A mutation allowing automatic notes after menu dismissal was rejected. Isolated AppKit/Computer Use acceptance confirmed Restore progress/completion visibility, automatic notes coexistence and menu persistence after closing the reader, explicit menu close, and global outside-click dismissal. FixtureTerminal is a separate no-command app; no real Restore or personal release acknowledgements were used. The first ambiguous preview result was traced to duplicate task-owned preview processes, then rechecked with a single instance. Parent-window screenshots clip floating popovers; AX and fixture window-event diagnostics are the evidence, not full-screen geometry or VoiceOver acceptance.
