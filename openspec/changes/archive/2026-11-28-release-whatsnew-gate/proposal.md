## Why

Desktop `Gtmux.app` What's New (`更新内容`) and `gtmux whatsnew` parse `user:` and `user-zh:` blocks from GitHub Release bodies, which GoReleaser populates from the annotated Git tag message. Unlike mobile release notes (`release-preflight.py`), `release.yml`'s `Resolve release source` gate (`scripts/macos_release_source.py`) did not verify on tag pushes that the tag is annotated, points to a green `main` CI commit, or carries non-empty `user:` and `user-zh:` blocks, allowing `v1.0.206` to publish with omitted What's New notes.

## What Changes

- Enforce in `scripts/macos_release_source.py` (`resolve` on `refs/tags/vX.Y.Z` pushes) that the tag is an annotated tag pointing to the exact `GITHUB_SHA` commit with passing `main` push CI and contiguous, non-empty `user:` and `user-zh:` blocks (rejecting missing headers, blank-line-split bullets, and code-fenced examples).
- Add `python3 scripts/macos_release_source.py --check-notes <path>` for pre-tag local verification and document the `user:` / `user-zh:` requirement in `AGENTS.md` and `docs/release-process.{md,zh.md}`.
- Add `scripts/macos_release_source.py` and `scripts/macos_release_source_test.py` to `QUALITY_ONLY_SCRIPTS` in `scripts/ci.py`.

## Capabilities

### Modified Capabilities
- `ci-checks`: Fail-closed release source gate requiring annotated tags, passing `main` CI, and bilingual `user:` / `user-zh:` What's New blocks before release jobs execute.

## Impact

## Surfaces

- **Terminal (`cmd/gtmux`)**: Guarantees every official release tag carries `user:` and `user-zh:` notes for `gtmux update` and `gtmux whatsnew`.
- **macOS menu-bar app (`macapp/`)**: Guarantees `WhatsNewController` (`更新内容`) receives bilingual entries on every release.
- **Phone (`mobileapp/`)**: Unchanged (mobile release notes continue to be gated by `release-preflight.py`).
- **iPad (`mobileapp/`)**: Unchanged.
- **Web (`internal/server/web/`)**: Unchanged.
