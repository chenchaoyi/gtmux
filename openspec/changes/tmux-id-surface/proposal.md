# tmux-id-surface — give every pane a stable, sigil'd tmux identity across all surfaces

Status: DESIGN CAPTURE — the core identity model is decided; scope/phasing is still under
discussion. Recorded here so the discussion can continue against a written baseline.

## Why

gtmux builds its whole pane identity on the MUTABLE coordinate — `loc =
session_name:window_index.pane_index`, plus a volatile display label (agent name / cleaned
title / cwd). The STABLE tmux ids (`$` session, `@` window, `%` pane) are almost never
surfaced. The result: no "global sense" — it is hard to map a pane to what it is actually
doing, or to map a terminal tab to a row in the app.

Two concrete symptoms:

- **The pane browser has no visible hierarchy.** Mobile and menu-bar collapse the window
  level into a `window.pane` chip (2-level: session → pane); only the web surface shows a
  `win N` divider. Rows are told apart by the volatile label + the mutable `window.pane`
  index + the cwd basename — none of it stable.
- **The Ghostty/iTerm2 tab carries no tmux id.** The tab title is `#S — #W` (session name —
  window name). The app describes a window by its INDEX; the tab describes it by its NAME.
  Those two axes never overlap, so a tab and an app row cannot be bridged even in principle.

What the data model carries today: `{ pane_id %N, session NAME, window INDEX, pane INDEX }`
plus a synthesized `loc`. It NEVER captures `session_id $N`, `window_id @N` (queried only
transiently for jumping, never stored), or `window_name` (only in the textual `overview`).
The one stable id that IS in the data — `pane_id %N` — is rendered on only one surface (the
menu-bar); mobile and web keep it as an internal key. (Note the trap: the existing
`agents --json` `session_id` field is the coding-agent ADOPT key, NOT tmux's `$N`.)

Observed on a real fleet (13 sessions / 21 panes) while designing this: 8 of 12 active
windows had window index `0` — under an index scheme every tab/row would read `…:0`,
indistinguishable; every plain shell pane's `pane_title` was the same host name string
(`ccy-MBP2024-…`) — proof that `pane_title` is not a usable per-pane name. The unique,
stable `@`/`%` ids are the only thing that tells them apart.

## What changes

Decided identity model — **id is the anchor, the name is the gloss; the sigil encodes the
level** (`$` session · `@` window · `%` pane), and the id is directly usable in commands
(`gtmux focus %11`):

- **Session** → its NAME (already unique per server, readable, and how you `attach -t`);
  `$id` optional/secondary.
- **Window** → `@id` + window name. The name drifts (tmux `automatic-rename` follows the
  running command) and is not unique, so `@id` is the anchor and the name is a gloss.
- **Pane** → `%id` + gtmux's OWN derived label (agent name + status, or command/dir). NOT
  `pane_title`, which is usually empty or the host name.

Applied to the surfaces:

- **Pane browser (mobile · menu-bar · web)** — render an explicit 3-level tree: session →
  `@id window-name` → `%id label`. Every tmux-level line wears its sigil'd id; hierarchy is
  carried by indentation AND the sigils. Surface `%N` on every row (menu-bar already does;
  mobile/web currently keep it as a key), tappable to copy `gtmux focus %N`.
- **Terminal tab title** — carry the ACTIVE pane's `%N` (`#S #{pane_id} #W`, e.g. `MP %11
  multipilot-companion`), because the pane is the unit of work and HQ refers to panes by
  `%N`; a tab showing only a window id could not be matched to what HQ names. A tab is a
  viewport onto the active window's active pane, so the `%N` is live (moves with focus).
- **tmux pane border** (`pane-border-format` / `pane-border-status`) — label each pane's
  border with `%N` so that in a SPLIT window every pane, including the non-active one, wears
  its id on screen. This is the piece that makes a pane findable when the tab can only name
  the one active pane (e.g. a window with an agent `%2` and a shell `%28` where the shell is
  active). Opt-in (it costs a row and changes the terminal's look) — suggested by `doctor`,
  not forced.
- **Data model** — additively capture `window_id @N` (and optionally `session_id $N`) in
  `PaneRow`/`agentJSON`; the new fields MUST be named to avoid colliding with the existing
  `session_id` adopt key (e.g. `win_id`, `tmux_session_id`).

Honest boundary: tmux ids are stable only within a server's life — `tmux kill-server`/reboot
can renumber `%N`. That is exactly the right scope for "watch the currently-running fleet";
these ids MUST NOT be used as a cross-restart persistent key.

## Open questions (to continue discussing)

- **Phasing.** Phase 1 (pure UI: show `%N` + real 3-level tree, no data/contract change),
  Phase 2 (the tab-title change — visible to every user, and coupled to the ghostty/iterm2
  title MATCHERS which parse `#S — #W`; `doctor`'s expected `set-titles-string` changes
  too), Phase 3 (capture `@N`/`$N`). Likely Phase 1 + 2 deliver the "global sense".
- **Tab granularity.** Active-pane `%N` (exact, but the title moves as focus moves within a
  window) vs window `@N` (calmer, but only maps to the window group, not the pane HQ names).
  Current lean: pane `%N` in the tab + `%N` on pane borders.
- **Session id.** Whether to also show/capture `$N`, or leave the session as name-only.
- **Does gtmux set names?** Optionally have gtmux assign stable window/pane names so the
  drifting `automatic-rename` gloss is more meaningful (spawn already does this for its own
  sessions) — a heavier "write into the user's tmux" behavior, deferred.
