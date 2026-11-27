## Why

Repository development rules, workflow requirements (OpenSpec SDD, test-design-first, bilingual docs, architecture invariants), and OpenSpec slash/skill instructions currently live only in `CLAUDE.md` and `.claude/`. When contributors work with other agent harnesses (Antigravity CLI `agy`, Codex, OpenCode, Kimi Code, Gemini CLI, Cursor, Copilot, DeepSeek Harness), those harnesses do not automatically load `CLAUDE.md` or `.claude/skills/`, causing missed workflow steps such as OpenSpec change proposals and archives.

## What changes

1. Consolidate all repository workflow, architecture, and contributor guidance into a single harness-agnostic root file `AGENTS.md`, putting the mandatory SDD + test-design-first + PR workflow up front with explicit thresholds for when an OpenSpec change proposal is required.
2. Turn `CLAUDE.md` and `GEMINI.md` into thin regular-file `@AGENTS.md` import pointers (no symlinks, preserving `scripts/ci.py`'s symlink ban) so Claude Code and Gemini CLI load the exact same rules without duplicating or drifting from `AGENTS.md`.
3. Provision OpenSpec skills in the cross-harness standard directory `.agents/skills/openspec-*/SKILL.md` (via `@fission-ai/openspec@1.10.0 init --tools agents`) alongside refreshed `.claude/{skills,commands/opsx}` artifacts, update `openspec/config.yaml` with current project context, and document direct `npx --yes @fission-ai/openspec@1.10.0 ...` CLI steps for harnesses without skill auto-discovery.
4. Add machine-checked conformance guards (`scripts/workflow_guard.py`, wired into `scripts/check-design.sh`) verifying `AGENTS.md` as the command-list & policy authority, thin regular-file pointers for `CLAUDE.md`/`GEMINI.md`, and cross-harness OpenSpec skill parity between `.agents/skills/` and `.claude/skills/`.
5. Add an `## OpenSpec` checklist section to `.github/pull_request_template.md`, update `scripts/ci.py` doc-path classification for `AGENTS.md`/`GEMINI.md`/OpenSpec skill files, and keep maintainer/docs references aligned.

## Non-goals

- Do not alter runtime product behavior, CLI commands, HTTP/SSE contracts, or stored user data formats.
- Do not introduce git symlinks (forbidden by `scripts/ci.py`).
- Do not block genuinely small spec clarifications in CI with brittle diff-size heuristics; enforce structural invariants mechanically and keep scope judgment in the PR review checklist.

## Surfaces

- Terminal (terminal / attach): No runtime CLI/attach behavior changes; contributor workflow and `scripts/check-design.sh` gate move their authority to `AGENTS.md`.
- Menubar (menubar): Not applicable — repository contributor workflow and CI policy only; no Swift or menu-bar changes.
- Phone (phone): Not applicable — repository contributor workflow and CI policy only; no mobile code changes.
- iPad: Not applicable — repository contributor workflow and CI policy only; no iPad code changes.
- Web: Not applicable — repository contributor workflow and CI policy only; no Web UI or server changes.
