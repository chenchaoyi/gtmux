# Tasks — HQ Charter productization (design now; build after forks resolved)

## Design (this change)
- [x] Proposal + promotion taxonomy + per-mechanism design + fork list
- [x] Resolve forks F1–F6 with the user (all per recommendation, 2026-07-13)
- [x] Spec deltas drafted (supervisor-agent seed playbook + watchdog; agent-dispatch reap-by-pane / titles / headless)

## PR 1 — seed promotion (M6, PROMPT-side)
- [x] `hqInstructions` carries B (responsiveness) + B2 (granularity) + C (window naming) + A (reclaim-is-HQ's-job), agent-neutral single-source
- [x] `hqKnowledgeSeeds` best-practices seeded with GENERIC operating lessons (F6a); concrete instances stay local
- [x] Tests: `TestHQPlaybookCharter` pins the charter markers; toward the in-flight change's supervisor-agent requirement

## PR 2 — reap-by-bare-pane (M1, D)
- [x] `gtmux reap <pane_id>` not in the ledger → derive worktree/branch from pane cwd (`dispatch.WorktreeContext`); same safety gate; kills the WINDOW not a session
- [x] Tests: `barePaneTask` synthesis, window-not-session kill, dirty report-only, window-only (main checkout), `WorktreeContext` linked-detection (real git)

## PR 3 — window titles (M3)  ·  PR 6 — headless spawn (M2)
- [x] `gtmux spawn` names the window+pane after a task slug (`--title` → worktree/branch leaf → goal head); pins `automatic-rename off` so it sticks; `--title` flag + tests
- [x] M2 `spawn --headless` (user chose: no terminal TAB, tracked): forces no-open, marks the window background (`⌁ `), still proxied/verified/tracked/reapable; seed references it; `windowName` test

## PR 4 — copy-mode injection guard (M4, G)
- [x] `internal/hqnudge`: `#{pane_in_mode}` short-circuits `boxEmpty` (treated like a non-empty draft) → queue, never inject; delivers on mode-exit/next drain
- [x] Test: in copy-mode → queued not sent; leaves mode → delivered

## PR 5 — dead-session / lifecycle watchdog (M5, D/G)
- [x] serve slow-tick: a pane stuck WAITING past the timeout → escalate to HQ, once per
  episode (presence marker, re-armed on leave), suggest-only, never about HQ itself
- [ ] finished/lingering-on-slow-tick incl. bare panes → reap-suggest — DEFERRED (the
  Stop-time reap-suggest sweep already covers ledgered dispatches; slow-tick + bare-pane
  extension is a small follow-up)
- [ ] working-with-no-output stuck detection — DEFERRED (needs last-activity tracking)

## Deferred to small follow-ups (NOT blocking archive)
- reap-suggest on the slow-tick for lingering/bare panes (the Stop-time sweep covers
  ledgered dispatches today)
- stuck-*working* detection (needs last-activity tracking)

## Close-out
- [x] Each shipped PR: make check + check-design green; spec+tests+docs same PR
- [x] M2 resolved (user: no-tab tracked) + spec deltas reconciled with what shipped
- [ ] Archive `hq-charter` (all six mechanisms landed; the two deferred items become
  their own future change)
