# Show Codex's pinned prompt in full in the browser mirror

## Why

Codex pins the prompt of the turn on screen to row 0 as one row cut at the pane's width with
"…". The phone now shows the full prompt in a bar (mobile-codex-pinned-prompt, #1285), but the
browser mirror still shows the cut row, and the rest of the prompt is nowhere in the capture:
Codex runs in the alternate screen. The user asked for the same on the web.

## What changes

- The browser's single-pane terminal view recognises the row with the phone's rules, unchanged:
  a Codex pane; row 0 a "› " row ending in "…" at the pane's right edge (`cols` from
  `GET /api/pane`); a "› " composer row below; exactly one of the ten latest logged prompts
  longer than the row and starting with it; the next row not continuing that prompt. When it
  matches, the full prompt goes in a bar above the terminal (two lines at rest, a click opens
  the rest, a Copy button) and the row is left out of what the terminal draws. Anything else is
  written to the terminal exactly as before.
- Cell widths come from the same tmux-measured table the phone uses (fix/emoji-cell-width).
- While the row is on screen but unexplained, the view fetches `GET /api/transcript` at most
  every 4 s, as the phone does; the terminal view did not read the log before.
- `app.js` carries a JavaScript copy of the matcher. A shared JSON fixture of cases runs
  against both copies, and a node test compares the two width tables, so neither drifts alone.

## Surfaces

- **终端 / terminal / attach**: not applicable — `gtmux attach` passes the PTY through, and Codex
  draws its own pinned row there.
- **菜单栏 / menubar**: not applicable — the menu bar has no terminal view.
- **手机 / phone**: already done in mobile-codex-pinned-prompt; this change adds the shared case
  fixture to its tests, no behavior change.
- **iPad**: already done (the same Detail component as the phone).
- **Web**: done for the single-pane terminal view. Not in the workbench tiles: a tile is a small
  multi-pane grid cell with no room for a second bar; its cut row stays as captured, and a
  click opens the single-pane view, which shows the bar.
