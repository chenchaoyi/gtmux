# tmux-id-surface — give every pane a stable, sigil'd tmux identity across all surfaces

Status: APPROVED — all three phases are in scope (commander, 2026-08-14). The window-name
question is settled and MEASURED; see "Decided by measurement" below.

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
(`dev-mbp.local`) — proof that `pane_title` is not a usable per-pane name. The unique,
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
- **Terminal tab title** — carries EVERY pane id in the window, through the window NAME.
  SUPERSEDED the original plan here (active pane's `%N` via a rewritten
  `set-titles-string`); see "Decided by measurement" below for what replaced it and why the
  measurement forced the change. `set-titles-string` is not touched at all.
- **tmux pane border** (`pane-border-format`) — now OPTIONAL rather than load-bearing. It
  was the only way a non-active pane could show its id; the window name carries them all
  now. Decide during Phase 2.
- **Data model** — additively capture `window_id @N` (and optionally `session_id $N`) in
  `PaneRow`/`agentJSON`; the new fields MUST be named to avoid colliding with the existing
  `session_id` adopt key (e.g. `win_id`, `tmux_session_id`).

Honest boundary: tmux ids are stable only within a server's life — `tmux kill-server`/reboot
can renumber `%N`. That is exactly the right scope for "watch the currently-running fleet";
these ids MUST NOT be used as a cross-restart persistent key.

## Decided by measurement (2026-08-14)

Two open questions closed, and neither answer was the one written above — both came from
probing a real tmux rather than reasoning about it.

### The tab carries EVERY pane id in its window, via the window NAME

Not the active pane's `%N` (the earlier lean), and not a change to `set-titles-string`.
The commander's shape: **the window name lists every `%N` in that window, independent of
focus**. `set-titles-string` stays `#S — #W`, so the tab inherits the ids for free.

    set -g automatic-rename-format '#{b:pane_current_path} #{P:#{pane_id} }'
    set-hook -g pane-exited 'set-window-option automatic-rename off ; set-window-option automatic-rename on'

Better than the earlier plan on three counts, and the last one is why Phase 2 gets much
cheaper:

- **Focus-independent.** An active-pane id changes as you move within a window; this does
  not. Measured: the name lagged the real active pane anyway (`name=[… %13]` while the
  active pane was `%14`) — tmux re-evaluates the format on its own schedule, so an
  active-pane id would have been a stale pointer dressed as a live one.
- **Complete.** In a split window every pane is named, not just the one in front — which
  was the whole reason the proposal wanted pane-border labels as a separate piece.
- **The title MATCHERS need no change at all.** The ids land after the `#S — ` separator,
  so `TitleMatchesSession` keeps working. Phase 2.2 as written (rewrite the matchers in
  lockstep) is no longer required — and note that the matchers were hardened for exactly
  this class of risk on 2026-08-13, when `tab-alert`'s `● ` prefix broke every jump.

Measured on an isolated server (`tmux -f /dev/null -L p2`):

| question | result |
|---|---|
| does `#{P:…}` iterate panes in a format? | **yes** — `#{P:#{pane_id} }` → `%0 %1 %2` |
| does adding a pane re-evaluate the name? | **yes** — split lands the new id immediately |
| does REMOVING a pane re-evaluate it? | **no** — name stuck at `%0 %1 %2` with `%0 %2` live |
| can a hook fix that? | **yes** — `pane-exited` toggling `automatic-rename` off/on; correct across consecutive kills |

So the removal half is the only part that needs anything, and a tmux hook covers it: **no
gtmux process is involved**. That keeps this a `doctor` SUGGESTION — two lines the user
owns and can revert — rather than gtmux writing into their tmux.

### gtmux does NOT rename windows

Deferred permanently, with evidence. `rename-window` turns `automatic-rename` OFF for that
window, so it would overwrite whatever format the user has. On the machine this was
designed against, `automatic-rename-format` is already `#{b:pane_current_path}` — windows
read `multipilot`, `sat-monitor`, `gtmux`. gtmux cannot guess better than that.

The default tmux format is `#{pane_current_command}`, which is where the "names drift"
premise came from — and for Claude 2.x that renders the VERSION STRING (`2.1.229`, the
#659 fact), so a default user's tab says `gtmux dev — 2.1.229`. That is a real problem,
but the lighter fix is `doctor` suggesting the format above, not gtmux renaming windows.
It also stays consistent with the identity model: `@id` is the anchor, the name is a
gloss, and a gloss is allowed to drift.

(`gtmux spawn` still names the windows IT creates — `spawn.go` `rename-window`, with the
`⌁ ` headless marker. Naming what you created is not the same as renaming what you found.)

### Probe hygiene — a footgun found while measuring this

The first probe used `TMUX_TMPDIR` + `-L` and WAS isolated, and still went wrong: a fresh
server reads the same `~/.tmux.conf`, so tmux-resurrect/continuum RESURRECTED the whole
saved fleet into the probe. Eleven sessions appeared out of nowhere and were nearly
mistaken for the live ones. **Always probe with `-f /dev/null`.**

## Open questions (to continue discussing)

- ~~**Phasing.**~~ SETTLED: all three phases are in scope.
- ~~**Tab granularity.**~~ SETTLED by measurement — every pane id, via the window name.
- ~~**Does gtmux set names?**~~ SETTLED: no. `doctor` suggests the format instead.
- **Session id.** Whether to also show/capture `$N`, or leave the session as name-only.
  Still open — the session NAME is already unique and is what `attach -t` takes, so `$N`
  has to earn its place rather than be added for symmetry.
- **Pane-border labels.** The proposal wanted `pane-border-format` `%N` so a split window
  shows every pane's id on screen. The window name now carries them all, so the border is
  no longer the only way to see a non-active pane's id — it is now a nice-to-have rather
  than the piece that makes the model work. Keep it opt-in, decide during Phase 2.
