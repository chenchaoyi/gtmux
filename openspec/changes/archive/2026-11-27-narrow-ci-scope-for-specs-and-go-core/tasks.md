## Tasks

- [x] Seal reviewed test design `docs/test/designs/ci-scope-speedup-20261127.json` before editing `scripts/ci.py` or `scripts/ci_test.py`.
- [x] Update `scripts/ci.py` and `scripts/ci_test.py` for `.openspec.yaml`, `quality`-only guard scripts, and scoped Go package downstream selection while keeping contracts/dependencies/`ci.py` fail-closed.
- [x] Update bilingual `docs/ci.md` and `docs/ci.zh.md` check-selection tables.
- [x] Validate and archive `narrow-ci-scope-for-specs-and-go-core` with OpenSpec 2.2.0 and verify all local gates (`ci_test.py`, `test_design.py --pr-history`, `check-design.sh --source-only`).
