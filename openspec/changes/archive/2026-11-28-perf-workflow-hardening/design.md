# Design: CI Scope Narrowing, Android Phone Sharding, and Workflow Hardening

## Decisions

1. **Incremental test-design history verification (`scripts/test_design.py`)**:
   - Load `old_mappings` from `base:docs/test/implementations.json` when available.
   - For a touched test file that already existed at `base`, compute `added_cases = [c for c in mapping['cases'] if c not in old_cases]`.
   - When `added_cases` is non-empty, require every case in `added_cases` to belong to a valid reviewed design already sealed in `commits[0]^` (and freshly sealed if that design file already existed at `base`), leaving untouched historical case IDs in `old_cases` intact.
   - When `added_cases` is empty, require at least one mapped case's design to have been newly added or re-sealed prior to `commits[0]`, preserving fail-closed protection against unreviewed edits to existing tests.

2. **Explicit Web-shared mobile source allowlist & mobile release metadata scope (`scripts/ci.py`)**:
   - Enumerate the exact `mobileapp/` paths consumed by `internal/server/web/` and `tests/web/` (`WEB_SHARED_MOBILE_PATHS` and `mobileapp/src/trace/`), selecting `web` when any of them changes while omitting `web` for ordinary `mobileapp/src/` and `mobileapp/e2e/` edits (and unit-test-only edits to non-shared RN files).
   - Classify `mobileapp/release-notes/*.srchash`, `mobileapp/release-notes/android.json`, and `mobileapp/ios/GtmuxMobile.xcodeproj/project.pbxproj` to their affected mobile/iOS/Android suites instead of `full = True`.

3. **Symmetric 2-shard Android phone & tablet verification (`scripts/android_ci_results.py`, `ci.yml`)**:
   - Extend `shard_attempts`, `selected_cases`, and `verify_device` to accept `--device {phone,tablet}` across shards `(1, 2)` while keeping `part == 0` full-suite verification available for local/ad-hoc runs.
   - Add `android-phone-shards` (`shard: [1, 2]`) feeding aggregate job `android-phone` (`name: Android phone emulator flows`), preserving exact downstream check-name compatibility with `delivery_source.py`.

4. **Single universal Mac Release build + `lipo` verification & SHA-pinned Node 24 actions**:
   - Remove the duplicate standalone `swift build -c release` and standalone `build-extension.sh` invocations from `ci.yml` `menubar`, relying on `GTMUX_UNIVERSAL=1 bash macapp/build.sh` (which already executes `Widget/build-extension.sh` and `Widget/verify_bundle.py`) plus an explicit `lipo -archs` check confirming both `arm64` and `x86_64` slices on `GtmuxBar`, `gtmux`, and `GtmuxUsageWidget`.
   - Pin `ios-store-assets.yml` Mac jobs to `macos-15` + `Xcode_26.3.app`, unify `setup-node` on Node `24`, and pin all third-party workflow actions to verified commit SHAs targeting the `node24` runtime.
