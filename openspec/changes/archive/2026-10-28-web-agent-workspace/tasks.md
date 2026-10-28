## Implementation
- [x] Unify focused and workbench conversation rendering, icons, copy, steps and spacing.
- [x] Enable scope-aware conversation input without losing drafts on transcript refresh.
- [x] Add searchable transcript trace with selectable details and copy.
- [x] Add authenticated loopback workspace entry to the menu bar.
- [x] Update user guidance and capability specs; verify browser flows and native build.

## Acceptance

- 55 JavaScript unit checks and 7 Chromium fixture flows pass, including desktop/narrow chat input, touch copy, clipboard fallback, trace filtering, draft retention and revocation cleanup.
- Go server/transcript race tests pass; the CLI builds with CGO disabled.
- Six Swift local workspace/remote recovery tests and the macOS release build pass.
- OpenSpec 1.10.0 validates all 38 specs. Design source checks pass in an isolated copy of the working tree; the original checkout's ignored sibling worktrees otherwise cause unrelated HOME-path false positives.
- Browser checks use random loopback ports and synthetic agents (1440×1000, 1280×850, 390×844, with shell widths down to 320px). The Browser plugin is absent; the repository's Playwright workflow was used.
- The installed/running app has not been replaced. Native menu clicking, Safari and real agent sending were not exercised. Trace exposes transcript summaries, not model request telemetry or full tool output.
