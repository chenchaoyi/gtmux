## 1. Native-session state store

- [x] 1.1 Add a `state` package area for native sessions — `NativeDir()` / `NativePath(sessionID)` under `~/.local/share/gtmux/native/` (document it in the state contract alongside `active/`, `waiting/`, `finished/`).
- [x] 1.2 Define the native record shape `{agent, sessionId, cwd, state, updatedAt}` (JSON) with read/write helpers; state values reuse the radar's working/waiting/idle vocabulary.

## 2. Hook: session-keyed path when outside tmux

- [x] 2.1 In `internal/hook/hook.go`, when `$TMUX_PANE` is empty AND a `session_id` is present, write/update the native record from the SAME `decide()` result (map the decision's set/clear to the record's state) instead of only doing a stateless notify.
- [x] 2.2 On `SessionEnd` (or equivalent), remove the native record.
- [x] 2.3 Unit-test the session-keyed branch: UserPromptSubmit→working, Stop→idle, waiting event→waiting, SessionEnd→removed — with `$TMUX_PANE` unset.

## 3. Radar: merge native rows into `agents --json`

- [x] 3.1 In `internal/app/agents.go`, read the native store and emit rows with `source: "native"`, no focusable locator, agent + project (cwd basename) + state.
- [x] 3.2 Idle "finished N ago" for native rows via `resume.Load`/`transcript.LastMessageTime` (session-keyed), consistent with tmux idle rows.
- [x] 3.3 De-dupe: suppress a native row whose `session_id` matches a live tmux pane (tmux wins).
- [x] 3.4 Reaping: omit native records past the staleness grace; keep idle-but-fresh ones. Decide the grace policy (SessionEnd + generous fallback).
- [x] 3.5 Tests: native rows present/absent, de-dupe against a tmux twin, stale record omitted, idle time source.

## 4. Focus/jump refusal for native rows

- [x] 4.1 Ensure `focus`/jump paths never attempt a terminal jump for a `source: "native"` identity (no locator); return a clear no-op/error.
- [x] 4.2 Test that a native session is not treated as a jump target.

## 5. Adopt-into-tmux (CLI/core)

- [ ] 5.1 Add a core action to adopt a native session: resolve its resume command (`internal/resume`) and spawn a fresh tmux session/window running it (reuse the `new`/`restore` spawn path).
- [ ] 5.2 Gate adoption on `resume.Resumable(agent)` + a captured `session_id`; expose eligibility so surfaces can hide Adopt for ineligible rows.
- [ ] 5.3 On adopt, mark the native record adopted so it drops from the native category once the tmux pane represents it.
- [ ] 5.4 Support adopting multiple sessions (each into its own window/session).
- [ ] 5.5 Tests: eligible adopt spawns the resume command; ineligible is refused; adopted record de-dupes/drops.

## 6. Menu-bar app: native category + Adopt action

- [ ] 6.1 In `macapp` (`AgentStore` + `MenuView`), parse `source: "native"` rows and render them in a dedicated labelled section ("Elsewhere" / "不在 tmux"), en+zh.
- [ ] 6.2 Native rows show no jump chevron / no reply; clicking does not focus.
- [ ] 6.3 Add a multi-select Adopt action with the duplicate-instance warning confirmation; call the CLI adopt path; hide Adopt for ineligible rows.
- [ ] 6.4 Verify the popover layout stays within the design (separate section, sense-only affordances) — swift build + on-device smoke.

## 7. Docs, specs, gate

- [ ] 7.1 Update `docs/design` + CLAUDE.md Scope note (native detection now IN scope as read-only sense + adopt; live view/input still out).
- [ ] 7.2 `make check` + `swift build -c release`; then `openspec validate --specs --strict` after sync/archive.
- [ ] 7.3 Sync/archive the change into `openspec/specs/` when landed.
