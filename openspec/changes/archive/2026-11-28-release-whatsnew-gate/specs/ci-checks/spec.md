## ADDED Requirements

### Requirement: Release source gate enforces annotated tags and bilingual What's New notes
Before building or publishing release artifacts on a `refs/tags/vX.Y.Z` push, `Resolve release source` (`scripts/macos_release_source.py`) SHALL verify that the tag is an annotated tag pointing to the exact `GITHUB_SHA` commit with passing `main` push CI and contiguous, non-empty `user:` and `user-zh:` What's New blocks without orphan bullets. `--check-notes <file>` SHALL validate tag notes locally before tagging.

#### Scenario: Tag push missing user: or user-zh: What's New blocks fails before builds
- **WHEN** `release.yml` runs on a `vX.Y.Z` tag that is lightweight, lacks passing `main` push CI, omits `user:` or `user-zh:`, or splits bullet lines across a blank line
- **THEN** `Resolve release source` fails closed before `goreleaser` or `menu-bar app (macOS)` executes

#### Scenario: Pre-tag local note validation
- **WHEN** an operator runs `python3 scripts/macos_release_source.py --check-notes <file>` on a release-notes file containing valid `user:` and `user-zh:` blocks
- **THEN** the check exits `0` and confirms both English and Chinese user-note counts
