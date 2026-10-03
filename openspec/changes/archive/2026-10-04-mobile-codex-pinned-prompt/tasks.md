## Implementation

- [x] Recognise Codex's pinned, truncated prompt (`ui/codexPinned.ts`) with a narrow match that
  leaves every other capture untouched; unit tests for the match and each refusal.
- [x] `PinnedPrompt` bar in the Detail chrome: two lines, tap to open, long press to copy; its
  height counted in the terminal's top padding and the chrome's slide.
- [x] Remove the Original/Wrap toggle and `sourceGridColumns`. (`cols` is read again after review, for
  a different reason: the cut row's right edge.)
- [x] Detail render tests: Codex gets the bar; the same screen from Claude Code, full screen and
  the chat are untouched.
- [x] MOBILE.md and MOBILE.zh.md, `api/contract.md`, the mobile-app spec.
- [x] Sync specs and archive this change.

## Review fixes (independent review of 63f05dd2)

- [x] Shape read off a real Codex 0.160.0 in throwaway 60-column panes: one row, newlines joined,
  cut to the width minus one cell, kept after the turn ends. The three-row match is gone.
- [x] M1: the terminal refetches the log while a cut row is unexplained, at most every 4 s;
  render probe A to B under an unchanged working status with B logged late.
- [x] L1: two different prompts that both explain the row leave it as captured; a prompt still
  being sent is not a candidate; same-prefix tests (unit and render).
- [x] L2: the row must end at the pane's right edge, the prompt must be longer than the row, and
  a next row continuing the prompt is a wrapped history message; tests for each.
- [x] L3: the bar's screen-reader label is the first 160 characters, "this turn's prompt";
  tests with 20,000 characters of multi-line Chinese and with characters outside the BMP.
- [x] L4: CODEX.md and CODEX.zh.md describe the phone terminal as built.

## Acceptance

Not claimed: a device or simulator check of the bar's layout under the chrome. Device and
live-app acceptance is recorded here, not as a task (CLAUDE.md, "Historical consistency").
