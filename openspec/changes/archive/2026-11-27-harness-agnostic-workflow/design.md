# Design: Harness-agnostic repo workflow (`AGENTS.md` + cross-harness OpenSpec)

## 1. Instruction Single Source of Truth

`AGENTS.md` becomes the canonical repository guide, matching the pattern `internal/hq/hq.go` already uses for HQ homes:

- `AGENTS.md` owns the complete repo map, mandatory pre-flight workflow (OpenSpec SDD → Test-Design-First → Verification → Archive → PR), build/verify commands, CLI command registry, and all design/conformance invariants.
- `CLAUDE.md` is a thin regular file containing `@AGENTS.md` (with a 1-line comment note), which Claude Code natively imports.
- `GEMINI.md` is a thin regular file containing `@AGENTS.md`, which Gemini CLI natively imports and Antigravity (`agy`) deduplicates alongside `AGENTS.md`.
- Neither pointer is a symlink because `scripts/ci.py` intentionally rejects symbolic links in diffs.

## 2. Cross-Harness OpenSpec Usability

Different coding-agent harnesses discover skills differently:

| Harness family | Rules file | Skill / workflow discovery |
| --- | --- | --- |
| Antigravity (`agy`), Codex, OpenCode, Kimi, Cursor, Copilot, Amp, Crush | `AGENTS.md` | `.agents/skills/openspec-*/SKILL.md` |
| Gemini CLI | `GEMINI.md` (`@AGENTS.md`) / `AGENTS.md` | `.agents/skills/openspec-*/SKILL.md` |
| Claude Code | `CLAUDE.md` (`@AGENTS.md`) | `.claude/skills/openspec-*/SKILL.md` + `.claude/commands/opsx/*.md` |
| Any harness without skill auto-discovery (e.g. Aider, DSH, shell) | `AGENTS.md` | Direct `npx --yes @fission-ai/openspec@1.10.0 <new\|status\|instructions\|validate\|archive>` commands documented in `AGENTS.md` |

Both `.agents/skills/` and `.claude/{skills,commands/opsx}` are generated/refreshed from the pinned `@fission-ai/openspec@1.10.0` toolchain (matching `scripts/openspec_check.py`). We deliberately do **not** add a third duplicate `.agent/skills/` copy because Antigravity (`agy`) natively discovers `.agents/skills/*/SKILL.md`.

`openspec/config.yaml` is refreshed so `openspec instructions` gives every harness accurate project context (five surfaces, `AGENTS.md` authority, bilingual docs, test-design-first).

## 3. Conformance Guards & CI Scoping

`scripts/workflow_guard.py` (invoked by `scripts/check-design.sh` and unit-tested by `scripts/workflow_guard_test.py`) verifies:
1. `AGENTS.md` exists as a non-empty regular file (not a symlink).
2. `CLAUDE.md` and `GEMINI.md` exist as regular files (not symlinks), contain `@AGENTS.md`, and stay thin pointers (≤ 10 non-empty lines, no duplicated `## ` headings).
3. All five core OpenSpec skills (`openspec-propose`, `openspec-apply-change`, `openspec-archive-change`, `openspec-explore`, `openspec-sync-specs`) exist with non-empty `SKILL.md` in both `.agents/skills/` and `.claude/skills/`.

`scripts/check-design.sh` points its CLI command registry check and `DOC_TREE` / retirement checks to `AGENTS.md` (keeping `CLAUDE.md` and `GEMINI.md` in `DOC_TREE` too).

`scripts/ci.py` adds `AGENTS.md`, `GEMINI.md`, `openspec/config.yaml`, `.agents/skills/**`, and `.claude/**` to its doc/workflow policy classification so editing instructions or OpenSpec skills triggers the `policy, docs and CI guards` job cleanly without falsely forcing full native iOS/Android builds.
