- [x] Implement shared disclosure glyphs and accessible whole-row actions.
- [x] Improve board outline, pane previews and expanded field presentation.
- [x] Preserve reading state and source content; add meaningful regressions.
- [x] Synchronize bilingual design/user docs, notes and main spec; archive.
- [x] Verify with repository/mobile/design gates and cached-debug simulator reading.

## Verification

- `GOFLAGS=-p=2 make check`: passed (vet, staticcheck, race tests).
- `npm run check -- --runInBand`: TypeScript, ESLint and all mobile unit suites passed; see PR for final totals. An initial sandbox run could not open loopback ports or inspect processes; the same gate passed with required local permissions.
- `bash scripts/check-design.sh`: passed including fixed-version strict OpenSpec validation.
- Reused an existing iPhone 17 Pro simulator debug binary with this branch's Metro bundle and synthetic board. English and Chinese controls, collapsed previews, expanded fields, full-text reveal and landscape layout were read from actual screenshots. Empty decision entry was absent; UI automation reached the field's expanded state. No native app build or live HQ board edit.
- Phone and iPad share this board reader. Physical-device reading and iPad visual acceptance remain pending; no claim that those were completed.
- Same-title duplicate insertion is disambiguated by occurrence, not a generated content ID. Renaming a heading is a new outline identity. Expansion persists for insertion of differently named headings/panes.
