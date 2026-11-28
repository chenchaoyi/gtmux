## 1. Implementation

- [x] 1.1 Partition `LiveScrollback` in `mobileapp/src/ui/LiveProgress.tsx` into memoized 32-row `<Text selectable>` chunks inside a vertical column `<View>`, wrapping lines wider than 320 cells via `wrapLine`.
- [x] 1.2 Add unit and component regression tests in `mobileapp/src/ui/LiveProgress.chunks.test.tsx` (`LVR-006`, `LVR-007`) and refresh test design catalogue mappings.
- [x] 1.3 Update paired design docs (`docs/design/MOBILE.md`, `docs/design/MOBILE.zh.md`) and stamp `1.0.206` release notes and version metadata for iOS and Android.
- [x] 1.4 Run `npm run check`, `./scripts/check-design.sh`, `release-preflight.py`, and `test_design.py --pr-history`.
