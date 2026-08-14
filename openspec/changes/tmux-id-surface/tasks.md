# Tasks — tmux-id-surface

Status: APPROVED, all three phases in scope (commander, 2026-08-14). Phase 2 got cheaper
after measurement — the tab carries every pane id through the window NAME, so
`set-titles-string` and the title matchers are not touched. See the proposal's
"Decided by measurement".

## Phase 1 — surface the stable anchor + real hierarchy (pure UI, no data/contract change)

- [ ] 1.1 Mobile pane browser (`mobileapp/src/screens/PaneBrowserScreen.tsx`): render an
      explicit window sub-group (`@id name`) under each session; show `%N` on every row.
- [x] 1.2 Menu-bar pane browser: window band (`@id name`) grouped by the STABLE id, shown
      only when a session has >1 window; every session header lists ALL its window ids so a
      COLLAPSED session still says what it holds; `%N` moved to the front of the row as the
      leading identifier and made searchable; the redundant `w.p` index dropped.
- [ ] 1.3 Web pane browser (`internal/server/web/`): show `%N` per row (3-level already).
- [ ] 1.4 One shared "identity chip" rendering (`session · @id name · %id label`) reused
      verbatim across the three surfaces; tap/click `%N` copies `gtmux focus %N`.
- [ ] 1.5 Tests: each surface renders `%N` + the window group (jest / Swift / web mirror).

## Phase 2 — tab ↔ app shared vocabulary (a doctor SUGGESTION, not a write)

Measured 2026-08-14: the window name can list every pane id, and that reaches the tab for
free because `set-titles-string` is `#S — #W`. gtmux writes nothing into the user's tmux.

- [ ] 2.1 `doctor` SUGGESTS the two lines (never applies them silently):
      `automatic-rename-format '#{b:pane_current_path} #{P:#{pane_id} }'` and the
      `pane-exited` hook that toggles `automatic-rename` off/on. The hook is REQUIRED, not
      cosmetic: adding a pane re-evaluates the name, removing one does NOT (measured — the
      name kept a dead `%1` while `%0 %2` were live).
- [ ] 2.2 `doctor` also flags the DEFAULT `automatic-rename-format`
      (`#{pane_current_command}`), which renders a Claude pane as its version string
      (`2.1.229`, #659) — the "names drift" problem, fixed by the same suggestion.
- [ ] 2.3 Pane-border `%N` (`pane-border-format`) — optional now that the window name
      carries every id. Decide whether it still earns its row.
- [ ] 2.4 Tests: the suggested format parses; `TitleMatchesSession` still matches a title
      whose window part carries ids (it must — that is why this shape was chosen).
- [ ] 2.5 NOT DONE: rewrite the title matchers. They need no change. Recorded so nobody
      re-adds the coupling the original plan assumed.

## Phase 3 — capture the stable ids (additive data)

- [x] 3.1 Add `#{window_id}` + `#{window_name}` to the pane query (`internal/radar/panes.go`)
      as `win_id` / `win_name`, additive + omitempty, appended to the END of the tmux format
      so the index-based parser is undisturbed. Named `win_id` to stay clear of the
      `session_id` adopt key. NOT added to `agents --json`: the radar is a flat agent list
      with no window level, so that field would be speculative until a consumer needs it.
- [ ] 3.2 Use `@N` for stable window grouping (a window keeps its identity across reorder).
- [x] 3.3 Tests (window ids distinguish two windows that BOTH have index 0 — the measured
      failure; a name shared by two windows is not an identity; absent fields stay absent)
      + `api/contract.md` note carrying the anchor/gloss rule and the server-lifetime
      boundary.

## Consistency (per the repo rule)

- [ ] X.1 Sync deltas into `openspec/specs/{pane-browser,agent-radar,terminal-jump}/spec.md`.
- [ ] X.2 Update `docs/design/{DESIGN,MOBILE}.md` and `docs/cli.md` where they describe the
      pane browser / tab titles; the state-language and `loc` conventions in CLAUDE.md.
- [ ] X.3 Archive this change once implemented.
