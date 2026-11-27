## Tasks

- [x] Seal reviewed test designs for `openspec_check.py`, `workflow_guard.py`, and `ci.py` before editing tests or mapped scripts.
- [x] Upgrade pinned OpenSpec CLI to `@fission-ai/openspec@2.2.0` across `scripts/openspec_check.py`, `scripts/check-design.sh`, and `AGENTS.md`.
- [x] Regenerate `.agents/skills/` and regular-file `.claude/skills/` (6 skills + `openspec/references/*.md`) from `2.2.0` and remove retired `.claude/commands/opsx/`.
- [x] Update `scripts/openspec_check.py`, `scripts/workflow_guard.py`, `scripts/ci.py`, unit tests, and `docs/test/` mappings/catalogue.
- [x] Validate `openspec-v2-upgrade` with `openspec validate --strict`, archive it, and pass `./scripts/check-design.sh --source-only` and `test_design.py --pr-history`.
