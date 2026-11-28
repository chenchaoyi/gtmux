## Context

`internal/app/changelog.go` parses `user:` and `user-zh:` marker blocks from GitHub Release bodies to populate `gtmux update`, `gtmux whatsnew`, and `Gtmux.app`'s What's New window. `release.yml` runs `Resolve release source` (`scripts/macos_release_source.py`) before `goreleaser` and `menu-bar app (macOS)`, but `resolve()` previously bypassed tag-object, `main` CI, and release-note checks on normal tag pushes.

## Goals / Non-Goals

**Goals:**
- Reject lightweight tags, commits without green `main` push CI, or tag annotations missing valid `user:` and `user-zh:` blocks in `Resolve release source` before any release asset is built or uploaded.
- Provide `--check-notes <path>` on `scripts/macos_release_source.py` for pre-tag local verification and document it in `AGENTS.md` and `docs/release-process.{md,zh.md}`.

**Non-Goals:**
- Changing `internal/app/changelog.go` runtime parsing format for historical releases.

## Decisions

- **Exact parser parity plus orphan-bullet guard**: Mirror `changelog.go`'s `markerLines` rules (outside ```` / `~~~` code fences, stopping at blank lines, `#` headings, PGP/SSH signature trailers, or the sibling marker), require both `user:` and `user-zh:` to be non-empty, and additionally reject bullet lines (`- ` / `* ` / `• `) sitting outside an open marker block before `## ` so accidental blank lines inside a block or missing headers fail closed.
- **Policy-scoped guard script**: Include `scripts/macos_release_source.py` and `scripts/macos_release_source_test.py` in `QUALITY_ONLY_SCRIPTS` since `policy` already runs `unittest discover -s scripts -p '*_test.py'`.
