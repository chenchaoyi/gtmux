# Show Codex's pinned prompt in full; drop the Original width toggle

## Why

Codex pins the prompt of the turn it is working on to the top row of its screen, cut to the
pane's width and ended with "…". The phone's Terminal tab showed that cut row, and the
bottom-left Original/Wrap toggle (#1217) was meant to help read wide Codex screens. It could
not help here: Codex runs in the alternate screen, so the rest of the prompt is nowhere in the
capture, wrapped or at the Mac's width. The user found the toggle useless and asked for Codex's
own presentation to be supported instead.

The conversation log the Chat tab already reads has the full prompt.

## What changes

- The Detail terminal recognises Codex's pinned, truncated prompt row and shows the full prompt
  from the conversation log in a bar at the bottom of the floating chrome: two lines at rest, a
  tap opens the rest, a long press copies it. The cut row leaves the terminal. The bar folds and
  returns with the rest of the chrome, and the terminal's top padding includes its height.
- Recognition is deliberately narrow: a Codex pane, row 0 starting with "› " and cut with "…"
  within three rows, what precedes the "…" beginning one of the ten most recent prompts
  (whitespace ignored), and a later "› " row (the composer) so the composer is never taken.
  Anything else leaves the capture byte for byte as it was, so Claude Code and every other agent
  render exactly as before. Not in full screen, where there is no chrome.
- The Original/Wrap toggle and its horizontal canvas are removed. The terminal wraps to the
  device as before. `GET /api/pane` keeps its optional `cols`.

## Surfaces

- **终端 / terminal / attach**: not applicable — `gtmux attach` passes the PTY through, and Codex
  draws its own pinned row there.
- **菜单栏 / menubar**: not applicable — the menu bar has no terminal view.
- **手机 / phone**: done — the bar under the Detail chrome; the toggle is removed.
- **iPad**: done — the regular shell renders the same Detail component.
- **Web**: not in this change — the browser mirror shows the captured screen as is, with the
  same cut row; a candidate follow-up if it matters there.
