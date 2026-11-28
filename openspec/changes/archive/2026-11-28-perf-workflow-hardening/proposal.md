# Proposal: CI Scope Narrowing, Android Phone Sharding, and Workflow Supply-Chain Hardening

## Why

A repository-wide workflow audit surfaced several high-ROI gaps:
1. `main` lacked a GitHub Ruleset binding the existing `ci-required` aggregate check.
2. `scripts/test_design.py --pr-history` rejected untouched historical case IDs when an existing test file appended a newly reviewed case, forcing either loss of historical trace links or test-file fragmentation.
3. `scripts/ci.py` treated mobile release-prep metadata (`.srchash`, `android.json`, `GtmuxMobile.xcodeproj`) as `full = True` and ran the 12-minute `web` Playwright suite on non-shared RN UI edits; `menubar` compiled the universal Mac release multiple times in a single job (~18 minutes); and `android-phone` ran all 31 emulator scenarios serially (~12 minutes after an 8.5-minute build).
4. `ios-store-assets.yml` used the moving `macos-latest` image alias, Node versions drifted between 22 and 24 across mobile workflows alongside deprecated Node 20 GitHub Actions, `release.yml` / `release-mirror.yml` used mutable action tags instead of commit SHAs, and `make check` omitted `./scripts/check-design.sh --source-only`.

## What Changes

- Enforce `main` branch protection via GitHub Ruleset (`ci-required` required before merge, direct push and force-push blocked).
- Update `scripts/test_design.py` `history_errors` to verify newly added case IDs (or at least one re-sealed existing case when no case ID is added) in the commit preceding the first test edit while preserving unchanged historical case IDs.
- Narrow `scripts/ci.py` so mobile release metadata selects mobile/iOS/Android suites without unrelated Go/menubar/Worker jobs, and non-shared `mobileapp/src/` UI files skip `web` while explicit Web-shared `mobileapp/` inputs continue selecting `web`.
- Split `android-phone` into two parallel emulator shards (`android-phone-shards` + aggregate `Android phone emulator flows`), eliminate duplicate universal Swift Release builds in `ci.yml` `menubar` while explicitly verifying `arm64` + `x86_64` product slices, and add `./scripts/check-design.sh --source-only` to `make check`.
- Pin `ios-store-assets.yml` to `macos-15` + `Xcode_26.3.app`, standardize mobile workflows on Node 24 LTS, and SHA-pin third-party GitHub Actions on Node 24 runtimes across all workflows.

## Surfaces

- **Terminal**: Unchanged runtime behavior; `make check` now runs `./scripts/check-design.sh --source-only` locally alongside Go gates.
- **Menubar**: CI `menubar` job verifies universal `arm64` + `x86_64` `GtmuxBar` and `GtmuxUsageWidget` binaries via `macapp/build.sh` without redundant pre-builds.
- **Phone**: Android phone CI runs in two verified parallel shards under the existing `Android phone emulator flows` check name; mobile release metadata and pure RN UI PRs skip unrelated suites.
- **iPad**: Shares `ios-native` and `ios-store-assets.yml` (`macos-15` + `Xcode_26.3.app` pinned); unaffected at runtime.
- **Web**: Still selected whenever `internal/server/web/`, `tests/web/`, `internal/server/`, `internal/artifact/`, or Web-shared `mobileapp/` sources change; skipped on non-shared RN UI edits.

## Impact

- Modified workflows & actions: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/workflows/release-mirror.yml`, `.github/workflows/ios-test-delivery.yml`, `.github/workflows/android-test-delivery.yml`, `.github/workflows/ios-store-assets.yml`, `.github/actions/android-emulator/action.yml`.
- Modified scripts, Makefile & docs: `Makefile`, `scripts/ci.py`, `scripts/ci_test.py`, `scripts/test_design.py`, `scripts/test_design_test.py`, `scripts/android_ci_results.py`, `scripts/android_ci_results_test.py`, `docs/release-process.md`, `docs/test/DESIGN-WORKFLOW.md`, `docs/test/DESIGN-WORKFLOW.zh.md`.
