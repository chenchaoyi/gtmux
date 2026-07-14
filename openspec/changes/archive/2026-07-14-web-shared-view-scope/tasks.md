## 1. Server — view scope + invariant (internal/server/share.go)

- [x] 1.1 Add `viewPanes map[string]bool` to `ShareManager`; persist as `view_panes` in the share.json state (seed from persisted state, tolerate missing → empty)
- [x] 1.2 Add `CanView(scope, pane)` — true for `full`, else `viewPanes[pane]`; keep `Allowed` (input) requiring consent + `panes[pane]`
- [x] 1.3 Enforce input ⊆ view in `SetConfig`/mutators: adding an input pane adds view; removing a view pane removes input. `share add %N` implies view
- [x] 1.4 Extend `GET /api/share` caller capability to `{input, view, panes, viewPanes}`
- [x] 1.5 Unit tests: invariant (add-input→view; remove-view→input), CanView per scope, default empty

## 2. Server — filter guest reads (internal/server/server.go + events.go)

- [x] 2.1 `handleAgents`: for guest scope, decode the agents array and drop non-viewable rows; `full` scope unchanged (byte-identical fast path)
- [x] 2.2 `handlePane`: refuse `403` when the caller is guest and the pane is not viewable
- [x] 2.3 SSE agents event: filter the pushed snapshot/delta by the subscriber's scope via `CanView`
- [x] 2.4 Web mirror endpoint(s): expose only viewable panes to a guest
- [x] 2.5 Decide + implement `/api/usage` + `/api/digest` for guest scope (proposed: refuse) per design Open Questions
- [x] 2.6 Tests: guest sees empty radar by default; guest sees only added view panes; `/api/pane` 403 for non-viewable; master/device unfiltered

## 3. CLI — `gtmux share view` (internal/app)

- [x] 3.1 Add `gtmux share view on|off|add %N|remove %N`; keep `share add %N` (input) implying view
- [x] 3.2 `gtmux share status` (+ `--json`) shows both allowlists; `status --json` carries `view_panes`
- [x] 3.3 en+zh i18n strings for the new subcommands; update `docs/cli.md`
- [x] 3.4 Tests for the new subcommands + the `--json` shape

## 4. Menu-bar — two controls per row (macapp)

- [x] 4.1 `ShareStore`: add `viewPanes` state + `setView(pane,on)` invoking `gtmux share view add/remove`; read `view_panes` from share.json / `--json`
- [x] 4.2 `Preferences.sharePanePicker`: per row add 👁 can-see + ⌨️ can-type; disable can-type unless can-see is on; reuse `shareablePanes` + `AgentAvatar`
- [x] 4.3 Update the LIVE-exposure indicator wording if needed (view vs input) and en/zh strings
- [x] 4.4 `GtmuxBarTests`: two-control binding + input-gated-by-view behavior

## 5. Spec sync + docs + release

- [ ] 5.1 `openspec validate --specs --strict` passes; archive the change after merge
- [x] 5.2 Update memory `web-shared-input` to note the view scope + secure default
- [ ] 5.3 Release notes call out the BREAKING guest-visibility default (fresh guest sees nothing)
