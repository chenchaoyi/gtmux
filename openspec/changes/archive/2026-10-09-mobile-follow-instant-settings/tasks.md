## Tasks

- [x] Use native switch appearance and stable rows with explicit master dependency.
- [x] Implement one revision-checked write per toggle, confirmed-state reconciliation on errors and late-response isolation.
- [x] Remove Save/Cancel; add Done and stable loading/update/error feedback.
- [x] Cover permissions, revisions, failure/reload, duplicate inputs, dismissal and layout invariants.
- [x] Sync bilingual user/design docs and spec deltas.

## Acceptance

TypeScript and targeted Jest checks passed (2 suites, 17 tests). Full mobile checks passed
(149 suites, 1594 tests, 15 skipped); the parallel Jest worker exit warning did not recur
under `--runInBand --detectOpenHandles`, which reported no open handles. `make check` and `scripts/check-design.sh` also passed. No native build
or unlocked phone/iPad layout and VoiceOver acceptance was performed. This change uses
existing React Native switches and adds no native dependency or server migration.
