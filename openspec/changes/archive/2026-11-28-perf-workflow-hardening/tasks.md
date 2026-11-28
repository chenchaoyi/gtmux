## 1. Implementation

- [x] 1.1 Update `scripts/test_design.py` (`history_errors`), `scripts/test_design_test.py`, and paired `docs/test/DESIGN-WORKFLOW*.md` so incremental case additions on existing test files preserve historical case IDs.
- [x] 1.2 Narrow `scripts/ci.py` for mobile release metadata and Web-shared vs non-shared `mobileapp/src/` paths, add `android-phone-shards`, and update `scripts/ci_test.py`.
- [x] 1.3 Generalize `scripts/android_ci_results.py` and `.github/actions/android-emulator/action.yml` to 2-shard both phone and tablet suites under `Android phone emulator flows` and `Android tablet emulator flows`, with unit tests in `scripts/android_ci_results_test.py`.
- [x] 1.4 Remove duplicate universal Release builds in `ci.yml` `menubar` (adding explicit `lipo -archs` verification), pin `ios-store-assets.yml` to `macos-15` + `Xcode_26.3.app`, standardize Node 24 + SHA-pinned Node 24 actions across all workflows, and include `./scripts/check-design.sh --source-only` in `Makefile` `check`.
- [x] 1.5 Validate OpenSpec, refresh test catalogue, pass `make check`, `actionlint`, and `--pr-history`, and configure `main` GitHub Ruleset requiring `ci-required`.
