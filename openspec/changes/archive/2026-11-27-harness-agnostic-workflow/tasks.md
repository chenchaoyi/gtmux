## Tasks

- [x] 1. Consolidate canonical contributor instructions into `AGENTS.md` with an upfront mandatory OpenSpec + test-design-first + PR workflow section and clear non-trivial change criteria.
- [x] 2. Replace `CLAUDE.md` and create `GEMINI.md` as thin regular-file `@AGENTS.md` pointers.
- [x] 3. Generate cross-harness `.agents/skills/openspec-*/SKILL.md` and refresh `.claude/{skills,commands/opsx}` using pinned `@fission-ai/openspec@1.10.0`; update `openspec/config.yaml` with current five-surface project context.
- [x] 4. Add `scripts/workflow_guard.py` and `scripts/workflow_guard_test.py` enforcing `AGENTS.md` authority, thin regular-file pointers, and cross-harness OpenSpec skill parity; wire into `scripts/check-design.sh` and update CLI command / `DOC_TREE` / comment-language references.
- [x] 5. Update `scripts/ci.py` and `scripts/ci_test.py` so `AGENTS.md`, `GEMINI.md`, `openspec/config.yaml`, `.agents/skills/**`, and `.claude/**` are classified as policy/doc files; update `.github/pull_request_template.md` with an `## OpenSpec` section and update `CONTRIBUTING.md`.
- [x] 6. Run `python3 -m unittest discover -s scripts -p '*_test.py'`, `./scripts/check-design.sh --source-only`, and `python3 scripts/test_design.py --pr-history` to verify all gates pass.
