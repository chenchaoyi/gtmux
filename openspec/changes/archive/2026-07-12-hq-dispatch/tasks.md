# Tasks — hq-dispatch

## 1 · Delivery + LAYERED verify core (unit-testable, cgo-free)

- [x] 1.1 `tmux.Paste(pane, text)` — `load-buffer -b <buf> -` (stdin, byte-exact)
      then `paste-buffer -d -b <buf> -t <pane>`; no auto-Enter. Keep `SendText`
      for short control replies. Unit test the arg shape (no-bin path). ✅ (done)
- [x] 1.2 `internal/dispatch.Deliver` — the LAYERED verify state machine.
      Injectable `IO{Capture, Paste, Enter, ClearDraft, Events(sinceTs), Now}`.
      Flow: interlock → pre-snapshot → paste + structural fragment guard (③) → Enter →
      verify (PRIMARY: an `Events` `submit` whose head matches ⇒ landed; `queued`
      screen marker ⇒ queued; FALLBACK: two-frame-consistent screen-read for hook-less
      agents) → backoff re-Enter (②) → timeout→`{delivered:false, state:"failed",
      evidence}`. Returns `{Delivered, State, Evidence, Attempts}`,
      `State∈landed|queued|failed|refused-duplicate`.
- [x] 1.2a Table tests: hook-path happy (submit event matches → landed, no screen
      needed), fragment (③ → retry/fail), swallowed Enter (② → re-Enter recovers),
      empty-box-not-working (③ → failed), `queued` marker → queued, single-frame
      transient (⑩ → NOT declared failed until two frames agree), timeout → failed.
- [x] 1.3 Head-normalization helper (collapse whitespace, first ~40 runes) shared by
      the `UserPromptSubmit` event head, the draft-assert, and the fallback history
      match; test on re-wrapped lines.
- [x] 1.4 Structural input-region locator (find the input separator/box line, return
      draft-region vs history-region) so "❯ text" is unambiguously draft vs submitted;
      test on captured Claude/Codex frames (fixtures).
- [x] 1.5 Re-send interlock (incident ⑨): per-pane recent-send store
      `~/.local/share/gtmux/sends/<pane>.json` `{hash, ts}`; `Deliver` refuses an
      identical `hash` within `resendWindow` unless `Force`. `now` injected. Test:
      duplicate within window refused, `Force` overrides, lapsed window allows.

## 2 · `gtmux spawn`

- [x] 2.1 `cmdSpawn` flag parse + usage (`--pane/--worktree/--model/--agent/--cwd/
      --no-open/--timeout/--json`, positional goal). Register in `app.go`; help/usage.
- [x] 2.2 Pre-flight: proxy resolution + would-403 warning; `preflightResource()`;
      `limits.Get` → model suggestion when `--model` omitted (advisory, never overrides).
- [x] 2.3 Target pane: create session (`newSessionArgs`) or reuse `--pane`; `--worktree`
      → `git worktree add`; open unfocused tab unless `--no-open`.
- [x] 2.4 Launch through `agentenv.Wrap` (proxy by construction — fixes ①);
      `launchAgent(agent, model)` appends `--model`; wait-for-ready via a bounded
      poll of `#{pane_current_command}` leaving a bare shell; then `dispatch.Deliver`.
- [x] 2.5 `--json {task_id, pane_id, session, delivered, state, evidence}`; human
      summary otherwise. On `delivered:false` exit non-zero and print evidence.

## 3 · Dispatch + needs-you ledger

- [x] 3.1 `internal/dispatch` ledger: `Add/Load/List/Remove` over
      `~/.local/share/gtmux/tasks/<id>.json`; status DERIVED from live pane state on read.
- [x] 3.2 `gtmux tasks [--json]` — needs-you-first list of tracked dispatches (waiting /
      done-review first), live status. Register + help.
- [x] 3.3 Digest rows: additive `task`/`task_status` fields when the pane has a ledger
      entry (empty otherwise). Contract test: absent for untracked panes.
- [x] 3.4 Ledger records what `spawn` CREATED: `session`/`worktree`/`branch` (for reap).

## 3b · Dispatch lifecycle closure — `gtmux reap` (incident ⑦)

- [x] 3b.1 `gtmux reap <pane|task_id> [--abandon] [--keep-branch] [--json]`: safety
      gate (worktree clean via `git status --porcelain` + branch merged, unless
      `--abandon`) → REPORT-ONLY on fail (name the blockers, touch nothing); on pass
      kill session + `git worktree remove` + delete merged branch (unless
      `--keep-branch`) + clear the ledger entry. Register + help.
- [x] 3b.2 Reclaim-suggest: a tracked task idle-after-work past `reapIdleThreshold`
      AND (branch merged or no branch) → `reap-suggest` nudge to HQ naming the exact
      `gtmux reap` command; deduped. Evaluated with the other transitions.
- [x] 3b.3 Snooze (incident ⑧): `gtmux reap --snooze <pane|task_id> [--for <dur>]`
      stamps `snooze_until = now + reapSnoozeTTL` (`--for` overrides), reclaims
      NOTHING; the reclaim-suggest check in 3b.2 skips entries whose `snooze_until` is
      future; the stamp clears on reap. `now` passed in (cgo-free, testable).
- [x] 3b.4 Tests: safety gate blocks a dirty worktree / unmerged branch (report-only);
      `--abandon` overrides; a clean+merged dispatch reaps and clears the ledger;
      `--snooze` stamps + suppresses the suggestion until it lapses, deletes nothing.
      `git`/`tmux` calls injectable so the test needs no real repo/server.

## 4 · `gtmux send` default verify

- [x] 4.1 CLI `gtmux send` text path → `dispatch.Deliver` (fast-return on confirm);
      `--no-verify` escape; `--force` overrides the re-send interlock; `--key`
      unchanged. `POST /api/send` UNCHANGED (stays fast).
- [x] 4.2 Tests: the deliver-verify core is table-tested in `internal/dispatch`
      (incl. the unstructured-shell path so a plain `send` isn't broken); the
      events→Ev bridge + `hookEquipped` are unit-tested; `POST /api/send` is
      unchanged by inspection (server handler not touched). Real-tmux smoke: a
      verified `send` lands, an identical re-send is refused, `--force` overrides.

## 5 · Nudge — both edges + ledger auto-clear (incident ⑤)

- [x] 5.1 Resolved edge in `internal/hook`: on the `clearWaiting` transition where a
      waiting marker EXISTED (`hadWaiting`), read its kind first, then nudge HQ
      `[gtmux] resolved <loc> (<pane>) — was <kind>`. Fires on UserPromptSubmit /
      Resumed / Stop. Dedup = marker existence (removed on the edge → one-shot).
- [x] 5.2 Ledger status is DERIVED live, so a resolved wait settles by itself
      (waiting→working / →done) — nothing to persist-clear. `done` nudge on a tracked
      task's Stop. Gated on live HQ + `hqNudge`. (Nudge typing is tmux-integration —
      not unit-tested, same as the pre-existing `nudgeSupervisor`.)

## 5b · Turn-end response awareness + triage (incident ⑥)

- [x] 5b.1 `events.Record` += additive `summary` + `class`; on every `Stop` the hook
      fills `summary` (reply tail, same extraction as digest `last`) and `class`
      (`asking`/`report`). Deterministic question heuristic (last sentence ends
      `?`/`？`), unit-tested on fixtures (incl. code/quote-line stripping).
- [x] 5b.1a `UserPromptSubmit` records the prompt's normalized `head` in `summary`
      (Claude payload carries `prompt`) so dispatch verify (1.2 PRIMARY) matches it;
      route `PreCompact` through `classify` as a state-neutral lifecycle event so a
      `/compact` is confirmable from the stream. Tests: submit head recorded,
      PreCompact emitted, neither breaks the marker state machine.
- [x] 5b.2 Nudge PUSH only for attention-worthy turns: an `asking` turn-end
      (`[gtmux] asks <loc> (<pane>) — "<summary>"`) and a tracked task's completion;
      ordinary `report` turns are pull-only (`gtmux events --follow`). Gated on live
      HQ + `hqNudge`; a `Stop` clears waiting, so it never double-fires with the menu
      `Waiting` path. The `asking`/`report` split is unit-tested (`classifyReply`).

## 6 · HQ role constraints (built-in)

- [x] 6.1 `hqInstructions`: role boundary (no engineering work; dispatch via
      spawn/send; NEVER nav keys into a TUI; unreadable form → focus + ask user), the
      ledger discipline, the resolved→retract-stale-chase rule, the turn-end triage
      rule, reclaim = suggest→approve→execute (never auto-delete), and
      decline→`--snooze` (don't re-nag a kept dispatch). Bilingual.
- [x] 6.2 `hqKnowledgeSeeds`: strengthen `environment.md` (auto-proxy covers ONLY
      gtmux's launch path — a bare send-keys launch 403s); add a dispatch pitfall entry.

## 7 · Ship

- [x] 7.1 Docs: `docs/cli.md` + `CLAUDE.md` command list add `spawn` + `tasks`; note
      send default-verify.
- [x] 7.2 `make check` green (gofmt/vet/staticcheck/`go test -race`); `CGO_ENABLED=0
      go build ./cmd/gtmux` passes.
- [x] 7.3 Every incident is pinned as an ACCEPTANCE TEST (table tests in
      `internal/dispatch` + `internal/hook` + `internal/app`): ① proxy-by-construction
      (spawn always `agentenv.Wrap`); ② swallowed-Enter re-send; ③ fragment /
      empty-box → `delivered:false`; ④ HQ playbook forbids nav keys (seed text); ⑤
      resolved-edge nudge on `hadWaiting`+clearWaiting; ⑥ `classifyReply` asking/report
      + `asks` push; ⑦ reap safety gate (dirty/unmerged report-only, clean+merged
      reaps); ⑧ `--snooze` suppresses the suggestion; ⑨ re-send interlock refuses a
      duplicate; ⑩ hook-event-first + two-frame consistency. Real-tmux SMOKE additionally
      exercised ②③⑨ end-to-end (verified `send` lands, dup refused, `--force` overrides).
      Full live-HQ multi-agent dogfood deferred to on-device use.
- [x] 7.4 `sync-specs` + `archive-change`; `openspec validate --specs --strict` passes.
