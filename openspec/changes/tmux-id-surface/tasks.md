# Tasks — tmux-id-surface

Status: PROPOSED (not started). Phased; Phase 1+2 are the "global sense" win, Phase 3 is
additive. Phasing/scope still under discussion (see proposal Open questions).

## Phase 1 — surface the stable anchor + real hierarchy (pure UI, no data/contract change)

- [ ] 1.1 Mobile pane browser (`mobileapp/src/screens/PaneBrowserScreen.tsx`): render an
      explicit window sub-group (`@id name`) under each session; show `%N` on every row.
- [ ] 1.2 Menu-bar pane browser (`macapp/Sources/GtmuxBar/PaneBrowser.swift`): add the
      window sub-header (its "tree" comment is currently not rendered); `%N` already shown.
- [ ] 1.3 Web pane browser (`internal/server/web/`): show `%N` per row (3-level already).
- [ ] 1.4 One shared "identity chip" rendering (`session · @id name · %id label`) reused
      verbatim across the three surfaces; tap/click `%N` copies `gtmux focus %N`.
- [ ] 1.5 Tests: each surface renders `%N` + the window group (jest / Swift / web mirror).

## Phase 2 — tab ↔ app shared vocabulary (coupled change)

- [ ] 2.1 Change `set-titles-string` to carry the active pane id (`#S #{pane_id} #W`);
      update `doctor`'s expected value (`internal/app/doctor.go`, `doctorfix.go`).
- [ ] 2.2 Update the title MATCHERS in lockstep (`internal/ghostty/ghostty.go`,
      `internal/terminal/iterm2.go`) — they currently parse `#S — #W`; keep `FocusTab`
      session matching working.
- [ ] 2.3 Opt-in pane-border `%N` labels (`pane-border-format` / `pane-border-status`),
      suggested by `doctor`, so split windows show every pane's id.
- [ ] 2.4 Tests: title round-trips through the matchers; doctor installs/repairs the new
      option.

## Phase 3 — capture the stable ids (additive data)

- [ ] 3.1 Add `#{window_id}` (and optionally `#{session_id}`) to the radar/pane queries
      (`internal/radar/agents.go`, `panes.go`); emit as new fields NAMED to avoid the
      existing `session_id` adopt-key collision (`win_id` / `tmux_session_id`).
- [ ] 3.2 Use `@N` for stable window grouping (a window keeps its identity across reorder).
- [ ] 3.3 Tests + `agents --json`/`panes --json` contract note (additive, optional fields).

## Consistency (per the repo rule)

- [ ] X.1 Sync deltas into `openspec/specs/{pane-browser,agent-radar,terminal-jump}/spec.md`.
- [ ] X.2 Update `docs/design/{DESIGN,MOBILE}.md` and `docs/cli.md` where they describe the
      pane browser / tab titles; the state-language and `loc` conventions in CLAUDE.md.
- [ ] X.3 Archive this change once implemented.
