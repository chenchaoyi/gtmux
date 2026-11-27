## Why

The repository pinned `@fission-ai/openspec@1.10.0` while npm `latest` moved to `2.2.0`, which adds the sixth entry-point `openspec` skill (plus `references/`), updates all five action skills, retires legacy `.claude/commands/opsx/` slash-command stubs, and adds stricter `Purpose`, RFC-2119, and `WHEN`/`THEN` validation checks. Upgrading the pinned CLI, cross-harness skills, and conformance gates to `2.2.0` keeps local harnesses and CI aligned on the current release.

## What Changes

- Bump the pinned OpenSpec CLI from `1.10.0` to `2.2.0` across `scripts/openspec_check.py`, `scripts/check-design.sh`, and `AGENTS.md`.
- Regenerate `.agents/skills/` and regular-file `.claude/skills/` from `@fission-ai/openspec@2.2.0` (6 skills including `openspec` + `references/{answers,where-things-stand,without-skills}.md`, with zero symlinks) and remove retired `.claude/commands/opsx/*.md`.
- Update `scripts/workflow_guard.py` and `scripts/ci.py` (with unit tests) to enforce all six `2.2.0` skills and classify `openspec/references/*.md` as policy-only docs.
- Update `scripts/openspec_check.py` (with unit tests) so every `2.2.0 --strict` error and structural/Purpose/RFC-2119/scenario warning fails closed while tolerating legacy `>500`-character prose advisories in existing `openspec/specs/` files.

## Surfaces

- **CLI**: No runtime CLI change (`scripts/openspec_check.py`, `scripts/workflow_guard.py`, `scripts/check-design.sh`, `scripts/ci.py` CI/dev tooling only).
- **Web UI**: None.
- **Mobile app**: None.
- **macOS menu-bar app**: None.
- **Docs**: `AGENTS.md`, `openspec/specs/docs-conformance/spec.md`, `.agents/skills/*`, `.claude/skills/*`.

## Capabilities

### Modified Capabilities
- `docs-conformance`: Update `OpenSpec workflow skills are provisioned across agent harnesses` for OpenSpec `2.2.0`'s six regular-file skills (including `openspec` and its `references/`) and removal of legacy `.claude/commands/opsx/`.

## Impact

- Contributors and coding agents in Claude Code, Antigravity (`agy`), Codex, OpenCode, Kimi, Gemini, Cursor, and Copilot run the exact same `2.2.0` OpenSpec skills and validator as CI.
