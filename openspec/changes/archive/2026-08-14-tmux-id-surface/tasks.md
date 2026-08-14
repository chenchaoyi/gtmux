# Tasks — tmux-id-surface

Status: APPROVED, all three phases in scope (commander, 2026-08-14). Phase 2 got cheaper
after measurement — the tab carries every pane id through the window NAME, so
`set-titles-string` and the title matchers are not touched. See the proposal's
"Decided by measurement".

## Phase 1 — surface the stable anchor + real hierarchy (pure UI, no data/contract change)

- [x] 1.1 Mobile pane browser: window band (`@id name`) keyed by the stable id and
      interleaved into the flat SectionList; every session header lists all its window ids;
      the row chip carries `%N` instead of the mutable `w.p`; agent rows lead with the
      radar's task; ids are searchable. Grouping extracted to pure functions and tested.
- [x] 1.2 Menu-bar pane browser: window band (`@id name`) grouped by the STABLE id, shown
      only when a session has >1 window; every session header lists ALL its window ids so a
      COLLAPSED session still says what it holds; `%N` moved to the front of the row as the
      leading identifier and made searchable; the redundant `w.p` index dropped.
- [x] 1.3 Web pane browser: the divider was keyed on the mutable INDEX and read "WIN 0";
      it now groups on `@id`, renders as a tinted band with `@id name`, drops out for a
      single-window session, and the session header lists every window id. Rows lead with
      `%N` and (for agents) the radar's task; ids are searchable.
- [x] 1.4 Tap/click `%N` copies `gtmux focus %N` — on all three surfaces, each confirming
      IN PLACE (`✓ copied`, ~1.2s) because a copy with no feedback cannot be told from a
      missed tap. The touch is scoped to the id: the row still opens the pane. The command
      itself comes from one named function per surface (`PaneCommands.focus` /
      `paneFocusCommand` ×2) so the string a user pastes is the one a test asserts. The web
      needs a fallback — `navigator.clipboard` is ABSENT over a plain `http://<lan-ip>`,
      which is a real way to reach that surface. (The "one shared rendering reused verbatim"
      half is NOT possible across SwiftUI / RN / DOM — what is shared is the SHAPE and the
      rules, which each surface now implements identically. Verbatim reuse was a wish
      written before the three renderers were weighed.)
- [x] 1.5 Tests: Swift (window identity, id capping, search haystack) + jest (grouping,
      interleaving, capping) + the web verified against a LIVE /api/panes payload rather
      than a fixture written to match the code.

## Phase 2 — tab ↔ app shared vocabulary (a doctor SUGGESTION, not a write)

Measured 2026-08-14: the window name can list every pane id, and that reaches the tab for
free because `set-titles-string` is `#S — #W`. gtmux writes nothing into the user's tmux.

- [x] 2.1 `doctor` SUGGESTS the two lines (never applies them silently):
      `automatic-rename-format '#{b:pane_current_path} #{P:#{pane_id} }'` and the
      `pane-exited` hook that toggles `automatic-rename` off/on. The hook is REQUIRED, not
      cosmetic: adding a pane re-evaluates the name, removing one does NOT (measured — the
      name kept a dead `%1` while `%0 %2` were live).
- [x] 2.2 `doctor` also flags a window name derived from the foreground COMMAND, which
      renders a Claude pane as its version string (`2.1.229`, #659) — the "names drift"
      problem, fixed by the same suggestion. It asks what the format is BUILT FROM rather
      than comparing to a literal: tmux 3.7's real default is
      `#{?pane_in_mode,[tmux],#{pane_current_command}}#{?pane_dead,[dead],}`, so the first
      version of this row (comparing to a bare `#{pane_current_command}`) called every
      DEFAULT install "custom, left alone" and never fired where it was needed. Caught by
      the command-level test below, not by reading the code.
- [x] 2.3 Pane-border `%N` — DECIDED: NO row, not even optional. Measured: `pane-border-status`
      defaults to `off`, so suggesting a `pane-border-format` alone changes nothing; making it
      visible costs a permanent screen ROW per pane in every split window. That is a real,
      recurring price for information the window name now carries for free. Documented as a
      one-liner in `docs/cli.md` for anyone who wants it, and left there.
- [x] 2.4 Tests (`internal/app/paneids_test.go`) pin the two MEASURED facts the design rests
      on — the format must iterate panes (not follow focus) and the hook must toggle
      `automatic-rename` — plus the property that keeps the title matchers out of this: the
      ids never lead the title.
- [x] 2.5 NOT DONE: rewrite the title matchers. They need no change. Recorded so nobody
      re-adds the coupling the original plan assumed.

## Phase 3 — capture the stable ids (additive data)

- [x] 3.1 Add `#{window_id}` + `#{window_name}` to the pane query (`internal/radar/panes.go`)
      as `win_id` / `win_name`, additive + omitempty, appended to the END of the tmux format
      so the index-based parser is undisturbed. Named `win_id` to stay clear of the
      `session_id` adopt key. NOT added to `agents --json`: the radar is a flat agent list
      with no window level, so that field would be speculative until a consumer needs it.
- [x] 3.2 Use `@N` for stable window grouping — delivered by 1.1–1.3: all three surfaces key
      the group on `win_id`, falling back to `'idx:' + window` only when the field is absent
      (an older CLI serving a newer app).
- [x] 3.3 Tests (window ids distinguish two windows that BOTH have index 0 — the measured
      failure; a name shared by two windows is not an identity; absent fields stay absent)
      + `api/contract.md` note carrying the anchor/gloss rule and the server-lifetime
      boundary.

## Consistency (per the repo rule)

- [x] X.1 Synced into `openspec/specs/{pane-browser,agent-radar,terminal-jump}/spec.md`. The
      terminal-jump delta was REWRITTEN before syncing — it still specified the pre-measurement
      design (active-pane id in `set-titles-string`, matchers updated in lockstep, pane-border
      as a doctor suggestion), none of which shipped. The agent-radar delta was corrected to
      match 3.1: `panes --json` only, not `agents --json`.
- [x] X.2 `docs/design/DESIGN.md` §16 and `MOBILE.md` §3 carry the identity rule (id is the
      anchor, name is the gloss); `docs/cli.md` documents the two tmux lines, why the hook is
      required, and why pane borders are NOT suggested. CLAUDE.md needed nothing — it never
      described `loc` or the pane browser's row identity.
- [x] X.3 Archived.
