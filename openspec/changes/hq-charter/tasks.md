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

## PR 3 — window titles (M3)  ·  headless spawn (M2) DEFERRED
- [x] `gtmux spawn` names the window+pane after a task slug (`--title` → worktree/branch leaf → goal head); pins `automatic-rename off` so it sticks; `--title` flag + tests
- [ ] M2 `spawn --headless` — DEFERRED: "no tmux window" conflicts with land-verify (which needs a pane). Needs one design nod (windowless-tracked model) before building; surfaced to the user.

## PR 4 — copy-mode injection guard (M4, G)
- [x] `internal/hqnudge`: `#{pane_in_mode}` short-circuits `boxEmpty` (treated like a non-empty draft) → queue, never inject; delivers on mode-exit/next drain
- [x] Test: in copy-mode → queued not sent; leaves mode → delivered

## PR 5 — dead-session / lifecycle watchdog (M5, D/G)
- [ ] serve slow-tick: finished/lingering (incl. bare panes) → reap-suggest; stuck/timed-out → escalate
- [ ] Deduped/snoozeable; suggest-only; tests

## Close-out
- [ ] Each PR: make check + check-design green; spec+tests+docs same PR
- [ ] Archive `hq-charter` after the last PR
