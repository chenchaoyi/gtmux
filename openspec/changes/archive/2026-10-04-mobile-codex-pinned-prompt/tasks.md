## Implementation

- [x] Recognise Codex's pinned, truncated prompt (`ui/codexPinned.ts`) with a narrow match that
  leaves every other capture untouched; unit tests for the match and each refusal.
- [x] `PinnedPrompt` bar in the Detail chrome: two lines, tap to open, long press to copy; its
  height counted in the terminal's top padding and the chrome's slide.
- [x] Remove the Original/Wrap toggle, `sourceGridColumns` and the `paneCols` plumbing.
- [x] Detail render tests: Codex gets the bar; the same screen from Claude Code, full screen and
  the chat are untouched.
- [x] MOBILE.md and MOBILE.zh.md, `api/contract.md`, the mobile-app spec.
- [x] Sync specs and archive this change.

## Acceptance

Not claimed: a device or simulator check of the bar's layout under the chrome. Device and
live-app acceptance is recorded here, not as a task (CLAUDE.md, "Historical consistency").
