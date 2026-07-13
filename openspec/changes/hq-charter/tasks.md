# Tasks — HQ Charter productization (design now; build after forks resolved)

## Design (this change)
- [x] Proposal + promotion taxonomy + per-mechanism design + fork list
- [x] Resolve forks F1–F6 with the user (all per recommendation, 2026-07-13)
- [x] Spec deltas drafted (supervisor-agent seed playbook + watchdog; agent-dispatch reap-by-pane / titles / headless)

## PR 1 — seed promotion (M6, PROMPT-side)
- [ ] `hqInstructions` rewritten to carry A–H + B2 as agent-neutral policy (single-source)
- [ ] `hqKnowledgeSeeds` best-practices seeded with GENERIC operating lessons (F6a)
- [ ] Tests: seed contains the charter policy markers; spec + docs

## PR 2 — reap-by-bare-pane (M1, D)
- [ ] `gtmux reap <pane_id>` resolves repo context from pane cwd when not in the ledger; same safety gate
- [ ] Tests: manual-window reclaim gated on clean+merged; dirty/unmerged report-only; `--json`

## PR 3 — window titles + headless spawn (M3, M2)
- [ ] `gtmux spawn` sets pane/window title (slug: `--title` → worktree/branch → goal head)
- [ ] `gtmux spawn --headless` (detached, no tmux window, proxied + tracked)
- [ ] Tests + naming convention in the seed

## PR 4 — copy-mode injection guard (M4, G)
- [ ] `internal/hqnudge`: treat `#{pane_in_mode}` like a non-empty draft (queue, deliver on exit)
- [ ] Tests: in-mode → queued not sent; exits mode → delivered

## PR 5 — dead-session / lifecycle watchdog (M5, D/G)
- [ ] serve slow-tick: finished/lingering (incl. bare panes) → reap-suggest; stuck/timed-out → escalate
- [ ] Deduped/snoozeable; suggest-only; tests

## Close-out
- [ ] Each PR: make check + check-design green; spec+tests+docs same PR
- [ ] Archive `hq-charter` after the last PR
