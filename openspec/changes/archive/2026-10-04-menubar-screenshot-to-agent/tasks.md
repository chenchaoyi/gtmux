## CLI

- [x] `gtmux send --attach FILE` (repeatable): content-named copy into the uploads dir,
  path on its own line, `attachments` in `--json`; refused with `--key`; nothing copied for
  a missing pane. Unit tests.
- [x] `gtmux panes --json` additive `viewed_at` from `tmux list-clients`; fixture test.

## Menu bar

- [x] Hotkey dispatch by id so a second hotkey (⌥⌘4) does not fire the palette.
- [x] Permission preflight, one request, and guidance with an Open System Settings button.
- [x] Capture through `screencapture -i`; cancel leaves nothing; popover closed first.
- [x] Editor window: arrow, rectangle, text, colours, undo/redo, Esc.
- [x] One export path for Copy (PNG + TIFF), Save (PNG) and Send.
- [x] Target picker with the viewed / last / active default; note field.
- [x] Send with re-read of the target, waiting refusal, `send --json` result mapping, no
  automatic retry.
- [x] Popover header button; Preferences lists both hotkeys.
- [x] Unit tests: hotkey dispatch, capture arguments and cancel, annotation undo, renderer
  at 1× and 2×, target default, send arguments and result mapping, waiting refusal.

## Docs and acceptance

- [x] docs/cli.md + zh (`send --attach`), api/contract.md (`viewed_at`), DESIGN.md + zh,
  TROUBLESHOOTING (Screen Recording and ad-hoc builds).
- [x] Real agents read a sent image on this Mac: Claude Code and Codex, through
  `send --attach`, judged by their replies, not by the path having been typed.
  (2026-10-04: a 640×320 PNG reading "MAGENTA 7391" in a red box. Claude Code "Read 1
  file" → `MAGENTA 7391`; Codex "Viewed image" → `MAGENTA 7391`. Re-sending the same image
  and note → `refused-duplicate`; a new note into a box holding a draft → `refused-draft`,
  the draft intact. Throwaway sessions, reaped after.)
- [x] Sync specs and archive this change.

## Acceptance

Not claimed: a real region capture through the hotkey or the popover button, and the
editor's clicks and keys (tools, undo, Esc, Copy, Save, Send) driven by a person. They need
Gtmux's Screen Recording grant, which only the user can give in System Settings, and no
one has clicked through the window. The logic behind them is unit-tested (18 tests) and the
CLI path to real agents was run end to end, as above. Known: an attachment refused at send
(draft, duplicate) has already been copied into the uploads dir; it is content-named, so it
is not duplicated, and is pruned with everything else there. Device and live-app acceptance
is recorded here, not as a task (CLAUDE.md, "Historical consistency").
