# Tasks — bilingual-playbook

Status: IMPLEMENTED (2026-08-17). The keeper is the anchor-set guard
(TestChartersShareSkeletonAndAnchors): the two charters cannot drift apart
silently.

- [x] 1. Split the charter: `hqInstructionsEN` (current text, Chinese asides
     removed) in hq.go; `hqInstructionsZH` (full translation, identical
     skeleton + anchors) in playbook_zh.go; `hqInstructions` aliases EN.
- [x] 2. Language-aware managed file: marker gains `lang:<en|zh>`;
     `generatedPlaybook()` picks the charter by `i18n.Lang()`; upgrade
     triggers on version OR language mismatch (backup keyed by
     version+lang; a legacy marker without lang regenerates once).
     `hqPlaybookVersion` 30 → 31.
- [x] 3. Guards: heading-sequence + backtick-anchor-set equality across the
     two charters; ban-list tests (author-leak, retired vocabulary) run over
     both; generatedPlaybook follows SetLang; a language switch upgrades a
     current-version install.
- [x] 4. Docs: docs/cli.md + README(+zh) say the charter follows GTMUX_LANG;
     spec delta (supervisor-agent playbook-language requirement).
- [x] 5. Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
     `scripts/check-design.sh`, `openspec validate --strict`.
