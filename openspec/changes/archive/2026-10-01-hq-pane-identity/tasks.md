## Implementation
- [x] Preserve authoritative HQ role in pane enumeration and adapters.
- [x] Mark confirmed HQ groups and panes across browsers; preserve custom names, search and targeting.
- [x] Name newly created dedicated HQ sessions Gtmux HQ; leave running sessions untouched.
- [x] Sync API contract, bilingual docs and specs.
- [x] Run meaningful role/name/interaction regression tests and required gates; archive and submit PR.

## Verification
- Go radar/HQ/CLI regressions and `GOFLAGS=-p=2 make check` passed. Tests use synthetic data and an isolated tmux server; the real HQ was not rotated or renamed.
- Mobile TypeScript/lint/Jest gate passed: 1,176 tests passed, 6 skipped. Real browser components exercised compact phone and regular iPad layouts, fold/search, and original pane targeting.
- Swift model/design tests passed (284); release build passed.
- Web Node tests passed (7), including the production pane renderer, filtered sibling identity and role-only repaint.
- Design/OpenSpec checks passed; bilingual CLI/design/mobile docs and the additive API contract are synchronized.
- Physical iPhone/iPad reading and pixel layout acceptance remains pending an unlocked connected device. No new mobile native build, release, installation or ASC update was performed.
