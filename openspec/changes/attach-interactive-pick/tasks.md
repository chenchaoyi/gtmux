# Tasks

## 1. Interactive pane picker (TTY)
- [x] 1.1 Factor pure helpers out of `pickPane`: `formatPaneChoice(Agent) string` (the
      one-line label) and `parsePaneChoice(input string, n int) (idx int, cancel bool, ok bool)`.
- [x] 1.2 In `pickPane`, when `len(panes) > 1` and stdin is a TTY, render the numbered
      menu, read a line (cooked mode), and resolve to a pane; loop re-prompt on invalid
      input; cancel on `q`/`Esc`/empty-EOF.
- [x] 1.3 Keep the non-TTY branch exactly as today (print list + return code 2).

## 2. Tests
- [x] 2.1 Unit-test `parsePaneChoice`: default (empty→row 1), valid number, out-of-range,
      `q`/`Q`, non-numeric, cancel.
- [x] 2.2 Unit-test `formatPaneChoice` for the row shape (session · agent · status · task,
      empty-field fallbacks).

## 3. Docs + spec
- [x] 3.1 `docs/cli.md` `gtmux attach`: document the interactive picker + the non-TTY
      fallback.
- [x] 3.2 Sync the `remote-terminal-client` spec (this change's delta) on archive.

## 4. Verify
- [x] 4.1 `make check` green; `openspec validate --specs --strict` passes.
