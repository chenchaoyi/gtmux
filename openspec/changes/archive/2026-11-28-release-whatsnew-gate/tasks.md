## 1. Implementation

- [x] 1.1 Enforce annotated tag, `main` push CI, and contiguous bilingual `user:` / `user-zh:` What's New blocks in `scripts/macos_release_source.py` (`resolve` and `--check-notes`), with unit tests in `scripts/macos_release_source_test.py`.
- [x] 1.2 Add `scripts/macos_release_source.py` and `scripts/macos_release_source_test.py` to `QUALITY_ONLY_SCRIPTS` in `scripts/ci.py` and test in `scripts/ci_test.py`.
- [x] 1.3 Document the `user:` / `user-zh:` annotated tag requirement and `--check-notes` preflight in `AGENTS.md` and `docs/release-process.{md,zh.md}`, archive OpenSpec, refresh test catalogue, and pass checks.
