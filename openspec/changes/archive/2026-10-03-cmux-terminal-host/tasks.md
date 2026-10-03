## Implementation

- [x] Distinguish cmux from embedded Ghostty in environment and process detection.
- [x] Add cmux focus, viewing, order, open-window, and workspace-spawn support
  through AppleScript, avoiding the CLI socket's external-caller limitation.
- [x] Add doctor and appearance integration and focused regression tests.
- [x] Update the terminal-jump spec and English/Chinese docs.
- [x] Run targeted tests, `make check`, and the design check.
- [x] Open PR and merge after CI passes. (#1250)

## Acceptance

Not claimed: focus, restore and viewing were not verified live with cmux running; #1250
shipped on its regression tests. Device and live-app acceptance is recorded here, not as a task (CLAUDE.md, "Historical consistency").
